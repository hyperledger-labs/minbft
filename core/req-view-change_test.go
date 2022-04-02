// Copyright (c) 2021 NEC Laboratories Europe GmbH.
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
	"testing"

	"github.com/golang/mock/gomock"
	mock_messagelog "github.com/hyperledger-labs/minbft/core/internal/messagelog/mocks"
	mock_viewstate "github.com/hyperledger-labs/minbft/core/internal/viewstate/mocks"
	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestMakeReqViewChangeValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	verify := func(msg messages.SignedMessage) error {
		args := mock.MethodCalled("messageSignatureVerifier", msg)
		return args.Error(0)
	}
	validate := makeReqViewChangeValidator(verify)

	rvc := messageImpl.NewReqViewChange(rand.Uint32(), 0)
	err := validate(rvc)
	assert.Error(t, err, "Invalid new view number")

	rvc = messageImpl.NewReqViewChange(rand.Uint32(), 1+uint64(rand.Int63()))

	mock.On("messageSignatureVerifier", rvc).Return(fmt.Errorf("invalid signature")).Once()
	err = validate(rvc)
	assert.Error(t, err, "Invalid signature")

	mock.On("messageSignatureVerifier", rvc).Return(nil).Once()
	err = validate(rvc)
	assert.NoError(t, err)
}

func TestMakeReqViewChangeProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	collect := func(rvc messages.ReqViewChange) (new bool, _ messages.ViewChangeCert) {
		args := mock.MethodCalled("reqViewChangeCollector", rvc)
		return args.Bool(0), args.Get(1).(messages.ViewChangeCert)
	}
	start := func(newView uint64, vcCert messages.ViewChangeCert) (ok bool, _ error) {
		args := mock.MethodCalled("viewChangeStarter", newView, vcCert)
		return args.Bool(0), args.Error(1)
	}
	process := makeReqViewChangeProcessor(collect, start)

	newView := rand.Uint64()
	rvc := messageImpl.NewReqViewChange(1, newView)
	cert := messages.ViewChangeCert{rvc, messageImpl.NewReqViewChange(2, newView)}

	mock.On("reqViewChangeCollector", rvc).Return(false, messages.ViewChangeCert(nil)).Once()
	new, err := process(rvc)
	assert.False(t, new)
	assert.NoError(t, err)

	mock.On("reqViewChangeCollector", rvc).Return(true, messages.ViewChangeCert(nil)).Once()
	new, err = process(rvc)
	assert.True(t, new)
	assert.NoError(t, err)

	mock.On("reqViewChangeCollector", rvc).Return(true, cert).Once()
	mock.On("viewChangeStarter", newView, cert).Return(false, nil).Once()
	new, err = process(rvc)
	assert.False(t, new)
	assert.NoError(t, err)

	mock.On("reqViewChangeCollector", rvc).Return(true, cert).Once()
	mock.On("viewChangeStarter", newView, cert).Return(true, nil).Once()
	new, err = process(rvc)
	assert.True(t, new)
	assert.NoError(t, err)
}

func TestMakeReqViewChangeCollector(t *testing.T) {
	const f = 1
	const n = 3

	var cases []struct {
		ID      uint32
		NewView uint64
		New     bool
		Cert    []int
	}
	casesYAML := []byte(`
- {id: 0, newview: 1, new: y, cert: []    }  #0
- {id: 0, newview: 1, new: n, cert: []    }  #1
- {id: 1, newview: 1, new: y, cert: [0, 2]}  #2
- {id: 2, newview: 1, new: n, cert: []    }  #3
- {id: 1, newview: 2, new: y, cert: []    }  #4
- {id: 0, newview: 3, new: n, cert: []    }  #5
- {id: 2, newview: 2, new: y, cert: [4, 6]}  #6
`)
	if err := yaml.UnmarshalStrict(casesYAML, &cases); err != nil {
		t.Fatal(err)
	}

	var msgs []messages.ReqViewChange
	var certs []messages.ViewChangeCert
	for _, c := range cases {
		rvc := messageImpl.NewReqViewChange(c.ID, c.NewView)
		msgs = append(msgs, rvc)

		var cert messages.ViewChangeCert
		for _, i := range c.Cert {
			cert = append(cert, msgs[i])
		}
		certs = append(certs, cert)
	}

	viewChangeCertSize := uint32(n - f)
	collect := makeReqViewChangeCollector(viewChangeCertSize)
	for i, c := range cases {
		desc := fmt.Sprintf("Case #%d", i)
		new, cert := collect(msgs[i])
		require.Equal(t, c.New, new, desc)
		require.Equal(t, certs[i], cert, desc)
	}
}

func TestMakeViewChangeStarter(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	id := rand.Uint32()
	viewState := mock_viewstate.NewMockState(ctrl)
	log := mock_messagelog.NewMockMessageLog(ctrl)
	startVCTimer := func(newView uint64) {
		mock.MethodCalled("viewChangeTimerStarter", newView)
	}
	handleGenerated := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	unprepareReqs := func() {
		mock.MethodCalled("requestUnpreparer")
	}
	start := makeViewChangeStarter(id, viewState, log, startVCTimer, unprepareReqs, handleGenerated)

	newView := rand.Uint64()
	cert := messages.ViewChangeCert{
		messageImpl.NewReqViewChange(1, newView),
		messageImpl.NewReqViewChange(2, newView),
	}
	msgs := []messages.Message{}
	viewLog := messages.MessageLog{}
	for cv := uint64(1); cv <= 2; cv++ {
		prep := messageImpl.NewPrepare(0, 0, messageImpl.NewRequest(0, cv, randBytes()))
		prep.SetUI(&usig.UI{Counter: cv, Cert: randBytes()})
		comm := messageImpl.NewCommit(1, prep)
		comm.SetUI(&usig.UI{Counter: cv, Cert: randBytes()})
		msgs = append(msgs, comm)
		viewLog = append(viewLog, comm)
	}
	req := messageImpl.NewRequest(0, 1, randBytes())
	msgs = append(msgs, nil)
	copy(msgs[2:], msgs[1:])
	msgs[1] = req

	viewState.EXPECT().AdvanceExpectedView(newView).Return(false, nil)
	ok, err := start(newView, cert)
	assert.False(t, ok)
	assert.NoError(t, err)

	viewState.EXPECT().AdvanceExpectedView(newView).Return(true, func() {
		mock.MethodCalled("viewReleaser")
	}).AnyTimes()

	vc := messageImpl.NewViewChange(id, newView, viewLog, cert)
	log.EXPECT().Messages().Return(msgs)
	log.EXPECT().Reset(nil)
	mock.On("viewChangeTimerStarter", newView)
	mock.On("requestUnpreparer").Once()
	mock.On("generatedMessageHandler", vc).Once()
	mock.On("viewReleaser").Once()
	ok, err = start(newView, cert)
	assert.True(t, ok)
	assert.NoError(t, err)
}
