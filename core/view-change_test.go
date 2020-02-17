// Copyright (c) 2022 NEC Laboratories Europe GmbH.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package minbft

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yaml "gopkg.in/yaml.v2"

	mock_messagelog "github.com/hyperledger-labs/minbft/core/internal/messagelog/mocks"
	mock_viewstate "github.com/hyperledger-labs/minbft/core/internal/viewstate/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/mocks"
	testifymock "github.com/stretchr/testify/mock"

	. "github.com/hyperledger-labs/minbft/messages/testing"
)

func TestMakeViewChangeValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	validateMessageLog := func(replicaID uint32, view uint64, log messages.MessageLog, nextCV uint64) error {
		args := mock.MethodCalled("messageLogValidator", replicaID, view, log, nextCV)
		return args.Error(0)
	}
	validateViewChangeCert := func(newView uint64, cert messages.ViewChangeCert) error {
		args := mock.MethodCalled("viewChangeCertValidator", newView, cert)
		return args.Error(0)
	}
	validate := makeViewChangeValidator(validateMessageLog, validateViewChangeCert)

	n := randN()
	replicaID := randReplicaID(n)
	view := randView()
	log := messages.MessageLog{}
	vcCert := messages.ViewChangeCert{}
	cv := rand.Uint64()
	vc := messageImpl.NewViewChange(replicaID, view+1, log, vcCert)
	vc.SetUI(&usig.UI{Counter: cv})

	mock.On("viewChangeCertValidator", view+1, vcCert).Return(fmt.Errorf("error")).Once()
	err := validate(vc)
	assert.Error(t, err, "Invalid view-change certificate")

	mock.On("viewChangeCertValidator", view+1, vcCert).Return(nil).Once()
	mock.On("messageLogValidator", replicaID, view, log, cv).Return(fmt.Errorf("error")).Once()
	err = validate(vc)
	assert.Error(t, err, "Invalid message log")

	mock.On("viewChangeCertValidator", view+1, vcCert).Return(nil).Once()
	mock.On("messageLogValidator", replicaID, view, log, cv).Return(nil).Once()
	err = validate(vc)
	assert.NoError(t, err)
}

func TestMakeViewChangeCertValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	const n, f = 3, 1
	const viewChangeCertSize = n - f

	validate := makeViewChangeCertValidator(viewChangeCertSize)

	newView := randView()
	cert := messages.ViewChangeCert{messageImpl.NewReqViewChange(randReplicaID(n), newView)}
	cert = append(cert, messageImpl.NewReqViewChange(randOtherReplicaID(cert[0].ReplicaID(), n), newView))

	err := validate(randOtherView(newView), cert)
	assert.Error(t, err, "Wrong view number")

	err = validate(newView, cert[:viewChangeCertSize-1])
	assert.Error(t, err, "Insufficient quorum")

	err = validate(newView, append(messages.ViewChangeCert{cert[0], cert[0]}, cert[2:]...))
	assert.Error(t, err, "Duplicate message in certificate")

	err = validate(newView, cert)
	assert.NoError(t, err)
}

func TestMakeMessageLogValidator(t *testing.T) {
	const n, f = 3, 1
	const maxView = 2
	const nrReqs = 2

	reqs := make([]messages.Request, nrReqs)
	for i := range reqs {
		reqs[i] = MakeTestReq(messageImpl, 0, uint64(i), randBytes())
	}

	for k := len(reqs); k >= 0; k-- {
		reqs := reqs[:k]
		for v := uint64(0); v <= maxView; v++ {
			i := 0
			for logs := range GenerateMessageLogs(messageImpl, f, n, v, reqs) {
				for r := uint32(0); r < n; r++ {
					t.Run(fmt.Sprintf("NrRequests=%d/View=%d/Log=%d/Replica=%d", k, v, i, r), func(t *testing.T) {
						testValidateMessageLog(t, r, n, v, logs[r])
					})
				}
				i++
			}
			if testing.Short() {
				return
			}
		}
	}
}

func testValidateMessageLog(t *testing.T, r, n uint32, v uint64, log messages.MessageLog) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nextCV := LastLogCV(log) + 1
	err := validateMessageLog(r, v, log, nextCV)
	assert.NoError(t, err)

	replaceMsg := func(log messages.MessageLog, i int, r uint32) messages.MessageLog {
		log = append(messages.MessageLog{}, log...)
		switch m := log[i].(type) {
		case messages.ViewChange:
			vc := messageImpl.NewViewChange(r, m.NewView(), m.MessageLog(), m.ViewChangeCert())
			vc.SetUI(m.UI())
			log[i] = vc
		case messages.NewView:
			nv := messageImpl.NewNewView(r, m.NewView(), m.NewViewCert())
			nv.SetUI(m.UI())
			log[i] = nv
		default:
			mm := mock_messages.NewMockCertifiedMessage(ctrl)
			mm.EXPECT().ReplicaID().Return(r).AnyTimes()
			mm.EXPECT().UI().Return(m.UI()).AnyTimes()
			log[i] = mm
		}
		return log
	}

	for i := range log {
		log2 := replaceMsg(log, i, randOtherReplicaID(r, n))
		err = validateMessageLog(r, v, log2, nextCV)
		assert.Error(t, err, "Foreign message in log")

		log2 = append(log[:i:i], log[i+1:]...)
		err = validateMessageLog(r, v, log2, nextCV)
		assert.Error(t, err, "Omitted message in log")

		if !mock.AssertExpectations(t) {
			t.FailNow()
		}
	}

	err = validateMessageLog(r, randOtherView(v), log, nextCV)
	assert.Error(t, err, "Wrong view")
}

func TestMakeViewChangeProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	r := randReplicaID(n)
	v := randOtherView(0)

	collectVC := func(vc messages.ViewChange) (new bool, nvCert messages.NewViewCert) {
		args := mock.MethodCalled("viewChangeCollector", vc)
		return args.Bool(0), args.Get(1).(messages.NewViewCert)
	}
	processNVCert := func(newView uint64, nvCert messages.NewViewCert) {
		mock.MethodCalled("newViewCertProcessor", newView, nvCert)
	}
	process := makeViewChangeProcessor(collectVC, processNVCert)

	vc1 := messageImpl.NewViewChange(r, v, nil, nil)
	mock.On("viewChangeCollector", vc1).Return(false, messages.NewViewCert(nil)).Once()
	new, err := process(vc1)
	assert.NoError(t, err)
	assert.False(t, new)

	vc2 := messageImpl.NewViewChange(r, v+1, nil, nil)
	mock.On("viewChangeCollector", vc2).Return(true, messages.NewViewCert(nil)).Once()
	new, err = process(vc2)
	assert.NoError(t, err)
	assert.True(t, new)

	vc3 := messageImpl.NewViewChange(randOtherReplicaID(r, n), v+1, nil, nil)
	cert := messages.NewViewCert{vc2, vc3}
	mock.On("viewChangeCollector", vc3).Return(true, cert).Once()
	mock.On("newViewCertProcessor", v+1, cert)
	new, err = process(vc3)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeViewChangeProcessorConcurrent(t *testing.T) {
	const n, f = 3, 1
	const r = 1
	const maxView = 42

	collectVC := makeViewChangeCollector(r, n, n-f)
	nvCerts := make(map[uint64]messages.NewViewCert)
	processNVCert := func(newView uint64, nvCert messages.NewViewCert) {
		nvCerts[newView] = nvCert
	}
	process := makeViewChangeProcessor(collectVC, processNVCert)

	var wg *sync.WaitGroup
	for v := uint64(1); v <= maxView; v++ {
		wg = new(sync.WaitGroup)
		if isPrimary(v, r, n) {
			wg.Add(n - f)
		}

		for id := uint32(0); id < n; id++ {
			go func(id uint32, v uint64, wg *sync.WaitGroup) {
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
				vc := messageImpl.NewViewChange(id, v, nil, nil)
				new, err := process(vc)
				assert.NoError(t, err)

				if new {
					wg.Done()
				}
			}(id, v, wg)
		}

		wg.Wait()
	}

	wg.Wait()
	for v := uint64(0); v <= maxView; v++ {
		if isPrimary(v, r, n) {
			assert.Contains(t, nvCerts, v)
			assert.NotNil(t, nvCerts[v])
		} else {
			assert.NotContains(t, nvCerts, v)
		}
	}
}

func TestMakeViewChangeCollector(t *testing.T) {
	const n, f = 4, 1
	const r = 1

	var cases []struct {
		ID   uint32
		View uint64
		New  bool
		Done bool
	}
	casesYAML := []byte(`
- {id: 1, view: 1, new: y, done: n}
- {id: 2, view: 1, new: y, done: n}
- {id: 0, view: 2, new: n, done: n}
- {id: 0, view: 5, new: y, done: n}
- {id: 3, view: 1, new: n, done: n}
- {id: 3, view: 5, new: y, done: n}
- {id: 1, view: 5, new: y, done: y}
- {id: 2, view: 5, new: n, done: n}
- {id: 3, view: 9, new: y, done: n}
- {id: 2, view: 9, new: y, done: n}
- {id: 0, view: 9, new: n, done: n}
- {id: 1, view: 9, new: y, done: y}
`)
	if err := yaml.UnmarshalStrict(casesYAML, &cases); err != nil {
		t.Fatal(err)
	}

	collect := makeViewChangeCollector(r, n, n-f)
	for i, c := range cases {
		desc := fmt.Sprintf("Case #%d", i)
		vc := messageImpl.NewViewChange(c.ID, c.View, nil, nil)
		new, cert := collect(vc)
		require.Equal(t, c.New, new, desc)
		if c.Done {
			require.NotNil(t, cert, desc)
		} else {
			require.Nil(t, cert, desc)
		}
	}
}

func TestMakeNewViewCertProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n := randN()
	r := randReplicaID(n)

	viewState := mock_viewstate.NewMockState(ctrl)
	log := mock_messagelog.NewMockMessageLog(ctrl)
	handleGenerated := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	process := makeNewViewCertProcessor(r, viewState, log, handleGenerated)

	cert := MakeTestNVCert(messageImpl)

	viewState.EXPECT().HoldView().Return(uint64(0), uint64(1), func() {
		mock.MethodCalled("viewReleaser")
	})
	log.EXPECT().Reset(nil)
	mock.On("generatedMessageHandler", messageImpl.NewNewView(r, 1, cert))
	mock.On("viewReleaser").Once()
	process(1, cert)

	viewState.EXPECT().HoldView().Return(uint64(1), uint64(2), func() {
		mock.MethodCalled("viewReleaser")
	})
	mock.On("viewReleaser").Once()
	process(1, cert)
}
