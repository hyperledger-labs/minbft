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
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
	"github.com/stretchr/testify/assert"

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
