// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
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

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
)

func TestMakePrepareValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	backup := randOtherReplicaID(primary, n)

	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: rand.Uint32(),
		},
	}
	ui := &usig.UI{Counter: rand.Uint64()}
	makePrepareMsg := func(id uint32) *messages.Prepare {
		return &messages.Prepare{
			Msg: &messages.Prepare_M{
				View:      view,
				ReplicaId: id,
				Request:   request,
			},
		}
	}

	verifyUI := func(msg messages.MessageWithUI) (*usig.UI, error) {
		args := mock.MethodCalled("uiVerifier", msg)
		return args.Get(0).(*usig.UI), args.Error(1)
	}
	validateRequest := func(request *messages.Request) error {
		args := mock.MethodCalled("requestValidator", request)
		return args.Error(0)
	}
	validate := makePrepareValidator(n, verifyUI, validateRequest)

	prepare := makePrepareMsg(backup)
	err := validate(prepare)
	assert.Error(t, err)

	prepare = makePrepareMsg(primary)

	mock.On("requestValidator", request).Return(fmt.Errorf("Invalid signature")).Once()
	err = validate(prepare)
	assert.Error(t, err)

	mock.On("requestValidator", request).Return(nil).Once()
	mock.On("uiVerifier", prepare).Return((*usig.UI)(nil), fmt.Errorf("UI not valid")).Once()
	err = validate(prepare)
	assert.Error(t, err)

	mock.On("requestValidator", request).Return(nil).Once()
	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	err = validate(prepare)
	assert.NoError(t, err)
}

func TestMakePrepareProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	otherView := randOtherView(view)
	primary := primaryID(n, view)
	id := randOtherReplicaID(primary, n)

	process := setupMakePrepareProcessorMock(mock, id)

	request := &messages.Request{
		Msg: &messages.Request_M{
			Seq: rand.Uint64(),
		},
	}
	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			View:      view,
			ReplicaId: primary,
			Request:   request,
		},
	}
	ownPrepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			View:      viewForPrimary(n, id),
			ReplicaId: id,
			Request:   request,
		},
	}
	commit := &messages.Commit{
		Msg: &messages.Commit_M{
			View:      view,
			ReplicaId: id,
			PrimaryId: primary,
			Request:   request,
		},
	}

	new, err := process(ownPrepare)
	assert.False(t, new)
	assert.NoError(t, err, "Own message")

	mock.On("requestProcessor", request).Return(false, fmt.Errorf("Error")).Once()
	_, err = process(prepare)
	assert.Error(t, err, "Failed to process Request")

	mock.On("requestProcessor", request).Return(false, nil).Once()
	mock.On("uiCapturer", prepare).Return(false).Once()
	new, err = process(prepare)
	assert.NoError(t, err)
	assert.False(t, new, "UI already processed")

	mock.On("requestProcessor", request).Return(false, nil).Once()
	mock.On("uiCapturer", prepare).Return(true).Once()
	mock.On("viewProvider").Return(otherView).Once()
	mock.On("uiReleaser", prepare).Once()
	_, err = process(prepare)
	assert.Error(t, err, "Prepare for different view")

	mock.On("requestProcessor", request).Return(false, nil).Once()
	mock.On("uiCapturer", prepare).Return(true).Once()
	mock.On("viewProvider").Return(view).Once()
	mock.On("requestSeqPreparer", request).Return(false).Once()
	mock.On("uiReleaser", prepare).Once()
	_, err = process(prepare)
	assert.Error(t, err, "Request ID already prepared")

	mock.On("requestProcessor", request).Return(false, nil).Once()
	mock.On("uiCapturer", prepare).Return(true).Once()
	mock.On("viewProvider").Return(view).Once()
	mock.On("requestSeqPreparer", request).Return(true).Once()
	mock.On("commitCollector", commit).Return(fmt.Errorf("Error")).Once()
	mock.On("uiReleaser", prepare).Once()
	assert.Panics(t, func() { process(prepare) }, "Failed to collect own Commit")

	mock.On("requestProcessor", request).Return(false, nil).Once()
	mock.On("uiCapturer", prepare).Return(true).Once()
	mock.On("viewProvider").Return(view).Once()
	mock.On("requestSeqPreparer", request).Return(true).Once()
	mock.On("commitCollector", commit).Return(nil).Once()
	mock.On("generatedUIMessageProcessor", commit).Once()
	mock.On("uiReleaser", prepare).Once()
	new, err = process(prepare)
	assert.NoError(t, err)
	assert.True(t, new)
}

func setupMakePrepareProcessorMock(mock *testifymock.Mock, id uint32) prepareProcessor {
	provideView := func() uint64 {
		args := mock.MethodCalled("viewProvider")
		return args.Get(0).(uint64)
	}
	captureUI := func(msg messages.MessageWithUI) (new bool, release func()) {
		args := mock.MethodCalled("uiCapturer", msg)
		return args.Bool(0), func() {
			mock.MethodCalled("uiReleaser", msg)
		}
	}
	processRequest := func(request *messages.Request) (new bool, err error) {
		args := mock.MethodCalled("requestProcessor", request)
		return args.Bool(0), args.Error(1)
	}
	prepareRequestSeq := func(request *messages.Request) (new bool) {
		args := mock.MethodCalled("requestSeqPreparer", request)
		return args.Bool(0)
	}
	collectCommit := func(commit *messages.Commit) error {
		args := mock.MethodCalled("commitCollector", commit)
		return args.Error(0)
	}
	handleGeneratedUIMessage := func(msg messages.MessageWithUI) {
		mock.MethodCalled("generatedUIMessageProcessor", msg)
	}
	return makePrepareProcessor(id, provideView, captureUI, prepareRequestSeq, processRequest, collectCommit, handleGeneratedUIMessage)
}
