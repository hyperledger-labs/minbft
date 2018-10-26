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
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logging "github.com/op/go-logging"
	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"

	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
	mock_messagelog "github.com/hyperledger-labs/minbft/core/internal/messagelog/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/mocks"
)

func TestMakeMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	validateMessage := func(msg interface{}) error {
		args := mock.MethodCalled("messageValidator", msg)
		return args.Error(0)
	}
	processMessage := func(msg interface{}) (new bool, err error) {
		args := mock.MethodCalled("messageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	replyMessage := func(msg interface{}) (reply <-chan interface{}, err error) {
		args := mock.MethodCalled("messageReplier", msg)
		return args.Get(0).(chan interface{}), args.Error(1)
	}
	handleMessage := makeMessageHandler(validateMessage, processMessage, replyMessage)

	msg := fmt.Sprint("message ", rand.Int())
	reply := fmt.Sprint("reply ", rand.Int())

	mock.On("messageValidator", msg).Return(fmt.Errorf("Error")).Once()
	_, _, err := handleMessage(msg)
	assert.Error(t, err)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, fmt.Errorf("Error")).Once()
	_, _, err = handleMessage(msg)
	assert.Error(t, err)

	nilRelyChan := (chan interface{})(nil)
	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	mock.On("messageReplier", msg).Return(nilRelyChan, fmt.Errorf("Error")).Once()
	_, new, err := handleMessage(msg)
	assert.Error(t, err)
	assert.False(t, new)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	mock.On("messageReplier", msg).Return(nilRelyChan, nil).Once()
	ch, new, err := handleMessage(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Nil(t, ch)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(true, nil).Once()
	mock.On("messageReplier", msg).Return(nilRelyChan, nil).Once()
	ch, new, err = handleMessage(msg)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.Nil(t, ch)

	replyChan := make(chan interface{}, 1)
	replyChan <- reply
	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	mock.On("messageReplier", msg).Return(replyChan, nil).Once()
	ch, new, err = handleMessage(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Equal(t, reply, <-ch)

	replyChan = make(chan interface{}, 1)
	replyChan <- reply
	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(true, nil).Once()
	mock.On("messageReplier", msg).Return(replyChan, nil).Once()
	ch, new, err = handleMessage(msg)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.Equal(t, reply, <-ch)
}

func TestMakeMessageValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	validateRequest := func(msg *messages.Request) error {
		args := mock.MethodCalled("requestValidator", msg)
		return args.Error(0)
	}
	validatePrepare := func(msg *messages.Prepare) error {
		args := mock.MethodCalled("prepareValidator", msg)
		return args.Error(0)
	}
	validateCommit := func(msg *messages.Commit) error {
		args := mock.MethodCalled("commitValidator", msg)
		return args.Error(0)
	}
	validateMessage := makeMessageValidator(validateRequest, validatePrepare, validateCommit)

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

	err := validateMessage(struct{}{})
	assert.Error(t, err, "Unknown message type")

	mock.On("requestValidator", request).Return(fmt.Errorf("Error")).Once()
	err = validateMessage(request)
	assert.Error(t, err, "Invalid Request")

	mock.On("requestValidator", request).Return(nil).Once()
	err = validateMessage(request)
	assert.NoError(t, err)

	mock.On("prepareValidator", prepare).Return(fmt.Errorf("Error")).Once()
	err = validateMessage(prepare)
	assert.Error(t, err, "Invalid Prepare")

	mock.On("prepareValidator", prepare).Return(nil).Once()
	err = validateMessage(prepare)
	assert.NoError(t, err)

	mock.On("commitValidator", commit).Return(fmt.Errorf("Error")).Once()
	err = validateMessage(commit)
	assert.Error(t, err, "Invalid Commit")

	mock.On("commitValidator", commit).Return(nil).Once()
	err = validateMessage(commit)
	assert.NoError(t, err)
}

func TestMakeMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	processRequest := func(msg *messages.Request) (new bool, err error) {
		args := mock.MethodCalled("requestProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processPrepare := func(msg *messages.Prepare) (new bool, err error) {
		args := mock.MethodCalled("prepareProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processCommit := func(msg *messages.Commit) (new bool, err error) {
		args := mock.MethodCalled("commitProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processMessage := makeMessageProcessor(processRequest, processPrepare, processCommit)

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

	_, err := processMessage(struct{}{})
	assert.Error(t, err, "Unknown message type")

	mock.On("requestProcessor", request).Return(false, fmt.Errorf("Error")).Once()
	_, err = processMessage(request)
	assert.Error(t, err, "Invalid Request")

	mock.On("requestProcessor", request).Return(false, nil).Once()
	new, err := processMessage(request)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("requestProcessor", request).Return(true, nil).Once()
	new, err = processMessage(request)
	assert.NoError(t, err)
	assert.True(t, new)

	mock.On("prepareProcessor", prepare).Return(false, fmt.Errorf("Error")).Once()
	_, err = processMessage(prepare)
	assert.Error(t, err, "Invalid Prepare")

	mock.On("prepareProcessor", prepare).Return(false, nil).Once()
	new, err = processMessage(prepare)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("prepareProcessor", prepare).Return(true, nil).Once()
	new, err = processMessage(prepare)
	assert.NoError(t, err)
	assert.True(t, new)

	mock.On("commitProcessor", commit).Return(false, fmt.Errorf("Error")).Once()
	_, err = processMessage(commit)
	assert.Error(t, err, "Invalid Commit")

	mock.On("commitProcessor", commit).Return(false, nil).Once()
	new, err = processMessage(commit)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("commitProcessor", commit).Return(true, nil).Once()
	new, err = processMessage(commit)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeMessageReplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	replyRequest := func(request *messages.Request) <-chan *messages.Reply {
		args := mock.MethodCalled("requestReplier", request)
		return args.Get(0).(chan *messages.Reply)
	}

	request := &messages.Request{
		Msg: &messages.Request_M{
			Seq: rand.Uint64(),
		},
	}
	reply := &messages.Reply{
		Msg: &messages.Reply_M{
			Seq: request.Msg.Seq,
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

	replyMessage := makeMessageReplier(replyRequest)

	_, err := replyMessage(struct{}{})
	assert.Error(t, err)

	replyChan := make(chan *messages.Reply, 1)
	replyChan <- reply
	mock.On("requestReplier", request).Return(replyChan).Once()
	ch, err := replyMessage(request)
	assert.NoError(t, err)
	assert.Equal(t, reply, <-ch)

	ch, err = replyMessage(prepare)
	assert.NoError(t, err)
	assert.Nil(t, ch)

	ch, err = replyMessage(commit)
	assert.NoError(t, err)
	assert.Nil(t, ch)
}

func TestMakeGeneratedUIMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	assignUI := func(msg messages.MessageWithUI) {
		mock.MethodCalled("uiAssigner", msg)
	}
	consume := func(msg interface{}) {
		mock.MethodCalled("messageConsumer", msg)
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

	mock.On("uiAssigner", msg).Run(func(args testifymock.Arguments) {
		m := args.Get(0).(messages.MessageWithUI)
		m.AttachUI(uiBytes)
	}).Once()
	mock.On("messageConsumer", msgWithUI).Once()
	handle(msg)
}

func TestMakeGeneratedUIMessageHandlerConcurrent(t *testing.T) {
	const nrMessages = 10
	const nrConcurrent = 5

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cv := uint64(0)
	log := make([]messages.MessageWithUI, 0, nrMessages*nrConcurrent)

	assignUI := func(msg messages.MessageWithUI) {
		cv++
		ui := &usig.UI{Counter: cv}
		uiBytes, _ := ui.MarshalBinary()
		mockMsg := msg.(*mock_messages.MockMessageWithUI)
		mockMsg.EXPECT().UIBytes().Return(uiBytes).AnyTimes()
	}
	consume := func(msg interface{}) {
		log = append(log, msg.(messages.MessageWithUI))
	}
	handle := makeGeneratedUIMessageHandler(assignUI, consume)

	wg := new(sync.WaitGroup)
	wg.Add(nrConcurrent)
	for i := 0; i < nrConcurrent; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < nrMessages; i++ {
				handle(mock_messages.NewMockMessageWithUI(ctrl))
			}
		}()
	}
	wg.Wait()

	assert.Len(t, log, nrMessages*nrConcurrent)
	for i, m := range log {
		ui := new(usig.UI)
		err := ui.UnmarshalBinary(m.UIBytes())
		if assert.NoError(t, err) {
			assert.EqualValues(t, i+1, ui.Counter)
		}
	}
}

func TestMakeMessageConsumer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientID := rand.Uint32()

	log := mock_messagelog.NewMockMessageLog(ctrl)
	clientState := mock_clientstate.NewMockState(ctrl)
	clientStates := func(id uint32) clientstate.State {
		require.Equal(t, clientID, id)
		return clientState
	}

	consume := makeMessageConsumer(log, clientStates, logging.MustGetLogger(module))

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
	reply := &messages.Reply{
		Msg: &messages.Reply_M{
			ReplicaId: rand.Uint32(),
			ClientId:  clientID,
			Seq:       rand.Uint64(),
		},
	}

	clientState.EXPECT().AddReply(reply).Return(nil)
	consume(reply)

	clientState.EXPECT().AddReply(reply).Return(fmt.Errorf("Invalid request ID"))
	assert.Panics(t, func() { consume(reply) })

	log.EXPECT().Append(msg)
	consume(prepare)
}
