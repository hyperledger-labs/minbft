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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/messages"

	mock_messagelog "github.com/hyperledger-labs/minbft/core/internal/messagelog/mocks"
)

func TestMakeMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	handleRequest := func(request *messages.Request, prepared bool) (reply <-chan *messages.Reply, new bool, err error) {
		args := mock.MethodCalled("requestHandler", request, prepared)
		return args.Get(0).(chan *messages.Reply), args.Bool(1), args.Error(2)
	}
	handlePrepare := func(prepare *messages.Prepare) (new bool, err error) {
		args := mock.MethodCalled("prepareHandler", prepare)
		return args.Bool(0), args.Error(1)
	}
	handleCommit := func(commit *messages.Commit) (new bool, err error) {
		args := mock.MethodCalled("commitHandler", commit)
		return args.Bool(0), args.Error(1)
	}

	handle := makeMessageHandler(handleRequest, handlePrepare, handleCommit)

	request := &messages.Request{
		Msg: &messages.Request_M{
			Seq: rand.Uint64(),
		},
	}
	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			Request: request,
		},
	}
	commit := &messages.Commit{
		Msg: &messages.Commit_M{
			Request: request,
		},
	}
	reply := &messages.Reply{
		Msg: &messages.Reply_M{
			Seq: request.Msg.Seq,
		},
	}

	assert.Panics(t, func() {
		handle(struct{}{})
	})

	var replyChan chan *messages.Reply
	err := fmt.Errorf("Failed to handle Request")
	mock.On("requestHandler", request, false).Return(replyChan, false, err).Once()
	_, _, err = handle(request)
	assert.Error(t, err)

	replyChan = make(chan *messages.Reply, 1)
	replyChan <- reply
	mock.On("requestHandler", request, false).Return(replyChan, false, nil).Once()
	ch, new, err := handle(request)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Equal(t, reply, <-ch)

	replyChan = make(chan *messages.Reply, 1)
	replyChan <- reply
	mock.On("requestHandler", request, false).Return(replyChan, true, nil).Once()
	ch, new, err = handle(request)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.Equal(t, reply, <-ch)

	err = fmt.Errorf("Failed to handle Prepare")
	mock.On("prepareHandler", prepare).Return(false, err).Once()
	_, _, err = handle(prepare)
	assert.Error(t, err)

	mock.On("prepareHandler", prepare).Return(false, nil).Once()
	_, new, err = handle(prepare)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("prepareHandler", prepare).Return(true, nil).Once()
	_, new, err = handle(prepare)
	assert.NoError(t, err)
	assert.True(t, new)

	err = fmt.Errorf("Failed to handle Commit")
	mock.On("commitHandler", commit).Return(false, err).Once()
	_, _, err = handle(commit)
	assert.Error(t, err)

	mock.On("commitHandler", commit).Return(false, nil).Once()
	_, new, err = handle(commit)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("commitHandler", commit).Return(true, nil).Once()
	_, new, err = handle(commit)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeGeneratedUIMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	assignUI := func(msg messages.MessageWithUI) {
		mock.MethodCalled("uiAssigner", msg)
	}
	consume := func(msg messages.MessageWithUI) {
		mock.MethodCalled("uiMessageConsumer", msg)
	}

	handle := makeGeneratedUIMessageHandler(assignUI, consume)

	msg := &messages.Prepare{
		Msg: &messages.Prepare_M{
			ReplicaId: rand.Uint32(),
		},
	}

	uiBytes := make([]byte, 1)
	rand.Read(uiBytes)
	msgWithUI := &messages.Prepare{
		Msg:       msg.Msg,
		ReplicaUi: uiBytes,
	}

	ch := make(chan messages.MessageWithUI, 1)
	ch <- msg
	close(ch)

	mock.On("uiAssigner", msg).Run(func(args testifymock.Arguments) {
		m := args.Get(0).(messages.MessageWithUI)
		m.AttachUI(uiBytes)
	}).Once()
	mock.On("uiMessageConsumer", msgWithUI).Once()
	handle(ch)
}

func TestMakeUIMessageConsumer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	log := mock_messagelog.NewMockMessageLog(ctrl)
	consumeUIMessage := makeUIMessageConsumer(log)

	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			View: rand.Uint64(),
		},
	}
	msg := &messages.Message{
		Type: &messages.Message_Prepare{
			Prepare: prepare,
		},
	}

	log.EXPECT().Append(msg)
	consumeUIMessage(prepare)
}
