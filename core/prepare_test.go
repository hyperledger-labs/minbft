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
	out := make(chan messages.MessageWithUI, 1)
	handle := setupMakePrepareHandlerMock(mock, id, n, view, out)

	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: rand.Uint32(),
		},
	}
	prepareUIBytes := make([]byte, 1)
	rand.Read(prepareUIBytes)
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

	var replyChan chan *messages.Reply

	prepare := makePrepareMsg(view, id)

	mock.On("uiAcceptor", prepare).Return(false, fmt.Errorf("Invalid UI")).Once()
	_, err := handle(prepare)
	assert.Error(t, err, "UI check failed")

	mock.On("uiAcceptor", prepare).Return(false, nil).Once()
	mock.On("requestHandler", request, true).Return(replyChan, false, nil).Once()
	new, err := handle(prepare)
	assert.NoError(t, err)
	assert.False(t, new, "UI already processed")

	prepare = makePrepareMsg(view+1, primaryID(n, view))
	mock.On("uiAcceptor", prepare).Return(true, nil).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Prepare is for different view")

	prepare = makePrepareMsg(view, randOtherReplicaID(id, n))
	mock.On("uiAcceptor", prepare).Return(true, nil).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Prepare creator is not primary in this view")

	prepare = makePrepareMsg(view, id)
	mock.On("uiAcceptor", prepare).Return(true, nil)

	err = fmt.Errorf("Invalid request ID")
	mock.On("requestHandler", request, true).Return(replyChan, false, err).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Invalid request ID")

	mock.On("requestHandler", request, true).Return(replyChan, true, nil)

	new, err = handle(prepare)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Empty(t, out)
}

func testMakePrepareHandlerBackup(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	id := randOtherReplicaID(primary, n)
	out := make(chan messages.MessageWithUI, 1)
	handle := setupMakePrepareHandlerMock(mock, id, n, view, out)

	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: rand.Uint32(),
		},
	}
	prepareUIBytes := make([]byte, 1)
	rand.Read(prepareUIBytes)
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

	var replyChan chan *messages.Reply

	prepare := makePrepareMsg(view, primary)

	mock.On("uiAcceptor", prepare).Return(false, fmt.Errorf("Invalid UI")).Once()
	_, err := handle(prepare)
	assert.Error(t, err, "UI check failed")

	mock.On("uiAcceptor", prepare).Return(false, nil).Once()
	mock.On("requestHandler", request, true).Return(replyChan, false, nil).Once()
	new, err := handle(prepare)
	assert.NoError(t, err)
	assert.False(t, new, "UI already processed")

	prepare = makePrepareMsg(view+1, primaryID(n, view))
	mock.On("uiAcceptor", prepare).Return(true, nil).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Prepare is for different view")

	prepare = makePrepareMsg(view, randOtherReplicaID(primary, n))
	mock.On("uiAcceptor", prepare).Return(true, nil).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Prepare creator is not primary in this view")

	prepare = makePrepareMsg(view, primary)
	mock.On("uiAcceptor", prepare).Return(true, nil)

	err = fmt.Errorf("Invalid request ID")
	mock.On("requestHandler", request, true).Return(replyChan, false, err).Once()
	_, err = handle(prepare)
	assert.Error(t, err, "Invalid request ID")

	mock.On("requestHandler", request, true).Return(replyChan, true, nil)

	mock.On("commitCollector", commit).Return(fmt.Errorf("Duplicated commit detected")).Once()
	assert.Panics(t, func() { _, _ = handle(prepare) }, "Failed collecting own Commit")
	assert.Empty(t, out)

	mock.On("commitCollector", commit).Return(nil)
	new, err = handle(prepare)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.Equal(t, commit, <-out)
}

func setupMakePrepareHandlerMock(mock *testifymock.Mock, id, n uint32, view uint64, out chan<- messages.MessageWithUI) prepareHandler {
	provideView := func() uint64 {
		args := mock.MethodCalled("viewProvider")
		return args.Get(0).(uint64)
	}
	acceptUI := func(msg messages.MessageWithUI) (new bool, err error) {
		args := mock.MethodCalled("uiAcceptor", msg)
		return args.Bool(0), args.Error(1)
	}
	handleRequest := func(request *messages.Request, prepared bool) (reply <-chan *messages.Reply, new bool, err error) {
		args := mock.MethodCalled("requestHandler", request, prepared)
		return args.Get(0).(chan *messages.Reply), args.Bool(1), args.Error(2)
	}
	collectCommit := func(commit *messages.Commit) error {
		args := mock.MethodCalled("commitCollector", commit)
		return args.Error(0)
	}
	mock.On("viewProvider").Return(view)
	return makePrepareHandler(id, n, provideView, acceptUI, handleRequest, collectCommit, out)
}
