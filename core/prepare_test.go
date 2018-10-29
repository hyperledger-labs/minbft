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

func TestMakePrepareHandler(t *testing.T) {
	t.Run("Primary", testMakePrepareHandlerPrimary)
	t.Run("Backup", testMakePrepareHandlerBackup)
}

func testMakePrepareHandlerPrimary(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	id := primaryID(n, view)
	handle := setupMakePrepareHandlerMock(mock, id, n, view)

	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: rand.Uint32(),
		},
	}
	ui := &usig.UI{Counter: rand.Uint64()}
	makePrepareMsg := func(view uint64, id uint32) *messages.Prepare {
		return &messages.Prepare{
			Msg: &messages.Prepare_M{
				View:      view,
				ReplicaId: id,
				Request:   request,
			},
		}
	}

	prepare := makePrepareMsg(view, id)

	mock.On("uiVerifier", prepare).Return((*usig.UI)(nil), fmt.Errorf("UI not valid")).Once()
	_, err := handle(prepare)
	assert.Error(t, err, "Faked own UI")

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	new, err := handle(prepare)
	assert.NoError(t, err)
	assert.False(t, new, "Own Prepare")

	otherPrimary := randOtherReplicaID(id, n)
	otherView := viewForPrimary(n, otherPrimary)
	prepare = makePrepareMsg(otherView, otherPrimary)

	mock.On("uiVerifier", prepare).Return((*usig.UI)(nil), fmt.Errorf("UI not valid")).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "UI not valid")

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(false, nil).Once()
	new, err = handle(prepare)
	assert.NoError(t, err)
	assert.False(t, new, "UI already processed")

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(true, nil).Once()
	mock.On("uiReleaser", prepare).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Prepare for different view")

	backup := randOtherReplicaID(id, n)
	prepare = makePrepareMsg(view, backup)

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(true, nil).Once()
	mock.On("uiReleaser", prepare).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Prepare is from backup replica")
}

func testMakePrepareHandlerBackup(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	id := randOtherReplicaID(primary, n)
	handle := setupMakePrepareHandlerMock(mock, id, n, view)

	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: rand.Uint32(),
		},
	}
	ui := &usig.UI{Counter: rand.Uint64()}
	prepareUIBytes, _ := ui.MarshalBinary()
	makePrepareMsg := func(view uint64, id uint32) *messages.Prepare {
		return &messages.Prepare{
			Msg: &messages.Prepare_M{
				View:      view,
				ReplicaId: id,
				Request:   request,
			},
			ReplicaUi: prepareUIBytes,
		}
	}
	commit := &messages.Commit{
		Msg: &messages.Commit_M{
			View:      view,
			ReplicaId: id,
			PrimaryId: primary,
			Request:   request,
			PrimaryUi: prepareUIBytes,
		},
	}

	prepare := makePrepareMsg(view, primary)

	mock.On("uiVerifier", prepare).Return((*usig.UI)(nil), fmt.Errorf("Invalid UI")).Once()
	_, err := handle(prepare)
	assert.Error(t, err, "UI check failed")

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(false, nil).Once()
	new, err := handle(prepare)
	assert.NoError(t, err)
	assert.False(t, new, "UI already processed")

	otherPrimary := randOtherReplicaID(id, n)
	otherView := viewForPrimary(n, otherPrimary)
	prepare = makePrepareMsg(otherView, otherPrimary)

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(true, nil).Once()
	mock.On("uiReleaser", prepare).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Prepare is for different view")

	backup := randOtherBackupID(id, n, view)
	prepare = makePrepareMsg(view, backup)

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(true, nil).Once()
	mock.On("uiReleaser", prepare).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Prepare is from another backup replica")

	prepare = makePrepareMsg(view, primary)

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(true, nil).Once()
	mock.On("requestHandler", request).Return(false, fmt.Errorf("Invalid request")).Once()
	mock.On("uiReleaser", prepare).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Invalid request")

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(true, nil).Once()
	mock.On("requestHandler", request).Return(true, nil).Once()
	mock.On("requestSeqPreparer", request).Return(fmt.Errorf("old request ID")).Once()
	mock.On("uiReleaser", prepare).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Old request ID")

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(true, nil).Once()
	mock.On("requestHandler", request).Return(true, nil).Once()
	mock.On("requestSeqPreparer", request).Return(nil).Once()
	mock.On("commitCollector", commit).Return(fmt.Errorf("Duplicated commit detected")).Once()
	mock.On("uiReleaser", prepare).Once()
	assert.Panics(t, func() { _, _ = handle(prepare) }, "Failed collecting own Commit")

	mock.On("uiVerifier", prepare).Return(ui, nil).Once()
	mock.On("uiCapturer", prepare).Return(true, nil).Once()
	mock.On("requestHandler", request).Return(true, nil).Once()
	mock.On("requestSeqPreparer", request).Return(nil).Once()
	mock.On("commitCollector", commit).Return(nil).Once()
	mock.On("generatedUIMessageHandler", commit).Once()
	mock.On("uiReleaser", prepare).Once()
	new, err = handle(prepare)
	assert.NoError(t, err)
	assert.True(t, new)
}

func setupMakePrepareHandlerMock(mock *testifymock.Mock, id, n uint32, view uint64) prepareHandler {
	provideView := func() uint64 {
		args := mock.MethodCalled("viewProvider")
		return args.Get(0).(uint64)
	}
	verifyUI := func(msg messages.MessageWithUI) (*usig.UI, error) {
		args := mock.MethodCalled("uiVerifier", msg)
		return args.Get(0).(*usig.UI), args.Error(1)
	}
	captureUI := func(msg messages.MessageWithUI) (new bool, release func()) {
		args := mock.MethodCalled("uiCapturer", msg)
		return args.Bool(0), func() {
			mock.MethodCalled("uiReleaser", msg)
		}
	}
	handleRequest := func(request *messages.Request) (new bool, err error) {
		args := mock.MethodCalled("requestHandler", request)
		return args.Bool(0), args.Error(1)
	}
	prepareRequestSeq := func(request *messages.Request) error {
		args := mock.MethodCalled("requestSeqPreparer", request)
		return args.Error(0)
	}
	collectCommit := func(commit *messages.Commit) error {
		args := mock.MethodCalled("commitCollector", commit)
		return args.Error(0)
	}
	handleGeneratedUIMessage := func(msg messages.MessageWithUI) {
		mock.MethodCalled("generatedUIMessageHandler", msg)
	}
	mock.On("viewProvider").Return(view)
	return makePrepareHandler(id, n, provideView, verifyUI, captureUI, prepareRequestSeq, handleRequest, collectCommit, handleGeneratedUIMessage)
}
