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

	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/usig"

	messages "github.com/hyperledger-labs/minbft/messages/protobuf"

	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
	mock_messagelog "github.com/hyperledger-labs/minbft/core/internal/messagelog/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/protobuf/mocks"
)

func TestMakeIncomingMessageHandler(t *testing.T) {
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
	handle := makeIncomingMessageHandler(validateMessage, processMessage, replyMessage)

	msg := fmt.Sprint("message ", rand.Int())
	reply := fmt.Sprint("reply ", rand.Int())

	mock.On("messageValidator", msg).Return(fmt.Errorf("Error")).Once()
	_, _, err := handle(msg)
	assert.Error(t, err)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, fmt.Errorf("Error")).Once()
	_, _, err = handle(msg)
	assert.Error(t, err)

	nilRelyChan := (chan interface{})(nil)
	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	mock.On("messageReplier", msg).Return(nilRelyChan, fmt.Errorf("Error")).Once()
	_, new, err := handle(msg)
	assert.Error(t, err)
	assert.False(t, new)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	mock.On("messageReplier", msg).Return(nilRelyChan, nil).Once()
	ch, new, err := handle(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Nil(t, ch)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(true, nil).Once()
	mock.On("messageReplier", msg).Return(nilRelyChan, nil).Once()
	ch, new, err = handle(msg)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.Nil(t, ch)

	replyChan := make(chan interface{}, 1)
	replyChan <- reply
	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	mock.On("messageReplier", msg).Return(replyChan, nil).Once()
	ch, new, err = handle(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Equal(t, reply, <-ch)

	replyChan = make(chan interface{}, 1)
	replyChan <- reply
	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(true, nil).Once()
	mock.On("messageReplier", msg).Return(replyChan, nil).Once()
	ch, new, err = handle(msg)
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

	assert.Panics(t, func() { validateMessage(struct{}{}) }, "Unknown message type")

	mock.On("requestValidator", request).Return(fmt.Errorf("Error")).Once()
	err := validateMessage(request)
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	processRequest := func(msg *messages.Request) (new bool, err error) {
		args := mock.MethodCalled("requestProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processReplicaMessage := func(msg messages.ReplicaMessage) (new bool, err error) {
		args := mock.MethodCalled("replicaMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makeMessageProcessor(processRequest, processReplicaMessage)

	request := &messages.Request{
		Msg: &messages.Request_M{
			Seq: rand.Uint64(),
		},
	}
	replicaMsg := mock_messages.NewMockReplicaMessage(ctrl)

	assert.Panics(t, func() { process(struct{}{}) }, "Unknown message type")

	mock.On("requestProcessor", request).Return(false, fmt.Errorf("Error")).Once()
	_, err := process(request)
	assert.Error(t, err, "Failed to process Request")

	mock.On("requestProcessor", request).Return(false, nil).Once()
	new, err := process(request)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("requestProcessor", request).Return(true, nil).Once()
	new, err = process(request)
	assert.NoError(t, err)
	assert.True(t, new)

	mock.On("replicaMessageProcessor", replicaMsg).Return(false, fmt.Errorf("Error")).Once()
	_, err = process(replicaMsg)
	assert.Error(t, err, "Failed to process replica message")

	mock.On("replicaMessageProcessor", replicaMsg).Return(false, nil).Once()
	new, err = process(replicaMsg)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("replicaMessageProcessor", replicaMsg).Return(true, nil).Once()
	new, err = process(replicaMsg)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeReplicaMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n := randN()
	id := randReplicaID(n)
	otherID := randOtherReplicaID(id, n)

	processMessage := func(msg interface{}) (new bool, err error) {
		args := mock.MethodCalled("messageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processUIMessage := func(msg messages.MessageWithUI) (new bool, err error) {
		args := mock.MethodCalled("uiMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makeReplicaMessageProcessor(id, processMessage, processUIMessage)

	replicaMsg := mock_messages.NewMockReplicaMessage(ctrl)
	uiMsg := mock_messages.NewMockMessageWithUI(ctrl)

	embeddedMsgs := []interface{}{
		&struct{ v int }{0},
		&struct{ v int }{1},
	}
	replicaMsg.EXPECT().EmbeddedMessages().Return(embeddedMsgs).AnyTimes()
	uiMsg.EXPECT().EmbeddedMessages().Return(embeddedMsgs).AnyTimes()

	replicaMsg.EXPECT().ReplicaID().Return(otherID)
	assert.Panics(t, func() { process(replicaMsg) }, "Unknown message type")

	uiMsg.EXPECT().ReplicaID().Return(id)
	new, err := process(uiMsg)
	assert.NoError(t, err)
	assert.False(t, new, "Own message")

	uiMsg.EXPECT().ReplicaID().Return(otherID).AnyTimes()

	mock.On("messageProcessor", embeddedMsgs[0]).Return(false, fmt.Errorf("Error")).Once()
	_, err = process(uiMsg)
	assert.Error(t, err, "Failed to process embedded message")

	mock.On("messageProcessor", embeddedMsgs[0]).Return(true, nil).Once()
	mock.On("messageProcessor", embeddedMsgs[1]).Return(false, fmt.Errorf("Error")).Once()
	_, err = process(uiMsg)
	assert.Error(t, err, "Failed to process embedded message")

	mock.On("messageProcessor", embeddedMsgs[0]).Return(false, nil)
	mock.On("messageProcessor", embeddedMsgs[1]).Return(true, nil)

	mock.On("uiMessageProcessor", uiMsg).Return(false, fmt.Errorf("Error")).Once()
	_, err = process(uiMsg)
	assert.Error(t, err, "Failed to process message with UI")

	mock.On("uiMessageProcessor", uiMsg).Return(false, nil).Once()
	new, err = process(uiMsg)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("uiMessageProcessor", uiMsg).Return(true, nil).Once()
	new, err = process(uiMsg)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeUIMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	captureUI := func(msg messages.MessageWithUI) (new bool, release func()) {
		args := mock.MethodCalled("uiCapturer", msg)
		return args.Bool(0), func() {
			mock.MethodCalled("uiReleaser", msg)
		}
	}
	processViewMessage := func(msg messages.ReplicaMessage) (new bool, err error) {
		args := mock.MethodCalled("viewMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makeUIMessageProcessor(captureUI, processViewMessage)

	uiMsg := mock_messages.NewMockMessageWithUI(ctrl)

	mock.On("uiCapturer", uiMsg).Return(false).Once()
	new, err := process(uiMsg)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("uiCapturer", uiMsg).Return(true).Once()
	mock.On("viewMessageProcessor", uiMsg).Return(false, fmt.Errorf("Error")).Once()
	mock.On("uiReleaser", uiMsg).Once()
	_, err = process(uiMsg)
	assert.Error(t, err, "Failed to process message in current view")

	mock.On("uiCapturer", uiMsg).Return(true).Once()
	mock.On("viewMessageProcessor", uiMsg).Return(false, nil).Once()
	mock.On("uiReleaser", uiMsg).Once()
	new, err = process(uiMsg)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("uiCapturer", uiMsg).Return(true).Once()
	mock.On("viewMessageProcessor", uiMsg).Return(true, nil).Once()
	mock.On("uiReleaser", uiMsg).Once()
	new, err = process(uiMsg)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeViewMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	waitView := func(view uint64) (ok bool, release func()) {
		args := mock.MethodCalled("viewWaiter", view)
		return args.Bool(0), func() {
			mock.MethodCalled("viewReleaser", view)
		}
	}
	applyReplicaMessage := func(msg messages.ReplicaMessage) error {
		args := mock.MethodCalled("replicaMessageApplier", msg)
		return args.Error(0)
	}
	process := makeViewMessageProcessor(waitView, applyReplicaMessage)

	view := randView()

	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			View: view,
		},
	}
	commit := &messages.Commit{
		Msg: &messages.Commit_M{
			View: view,
		},
	}

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockReplicaMessage(ctrl)
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("viewWaiter", view).Return(false).Once()
		new, err := process(prepare)
		assert.NoError(t, err)
		assert.False(t, new, "Message for former view")

		mock.On("viewWaiter", view).Return(true).Once()
		mock.On("replicaMessageApplier", prepare).Return(nil).Once()
		mock.On("viewReleaser", view).Once()
		new, err = process(prepare)
		assert.NoError(t, err)
		assert.True(t, new)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("viewWaiter", view).Return(false).Once()
		new, err := process(commit)
		assert.NoError(t, err)
		assert.False(t, new, "Message for former view")

		mock.On("viewWaiter", view).Return(true).Once()
		mock.On("replicaMessageApplier", commit).Return(nil).Once()
		mock.On("viewReleaser", view).Once()
		new, err = process(commit)
		assert.NoError(t, err)
		assert.True(t, new)
	})
}

func TestMakeReplicaMessageApplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	applyPrepare := func(msg *messages.Prepare) error {
		args := mock.MethodCalled("prepareApplier", msg)
		return args.Error(0)
	}
	applyCommit := func(msg *messages.Commit) error {
		args := mock.MethodCalled("commitApplier", msg)
		return args.Error(0)
	}
	apply := makeReplicaMessageApplier(applyPrepare, applyCommit)

	reqSeq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			Seq: reqSeq,
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
			Seq: reqSeq,
		},
	}

	msg := mock_messages.NewMockReplicaMessage(ctrl)
	assert.Panics(t, func() { apply(msg) }, "Unknown message type")

	mock.On("prepareApplier", prepare).Return(fmt.Errorf("Error")).Once()
	err := apply(prepare)
	assert.Error(t, err, "Failed to apply Prepare")

	mock.On("prepareApplier", prepare).Return(nil).Once()
	err = apply(prepare)
	assert.NoError(t, err)

	mock.On("commitApplier", commit).Return(fmt.Errorf("Error")).Once()
	err = apply(commit)
	assert.Error(t, err, "Failed to apply Commit")

	mock.On("commitApplier", commit).Return(nil).Once()
	err = apply(commit)
	assert.NoError(t, err)

	err = apply(reply)
	assert.NoError(t, err)
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

	assert.Panics(t, func() { replyMessage(struct{}{}) }, "Unknown message type")

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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	assignUI := func(msg messages.MessageWithUI) {
		mock.MethodCalled("uiAssigner", msg)
	}
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	handleGeneratedUIMessage := makeGeneratedUIMessageHandler(assignUI, handleGeneratedMessage)

	msg := mock_messages.NewMockMessageWithUI(ctrl)

	uiBytes := make([]byte, 1)
	rand.Read(uiBytes)
	msg.EXPECT().AttachUI(uiBytes)

	mock.On("uiAssigner", msg).Run(func(args testifymock.Arguments) {
		m := args.Get(0).(messages.MessageWithUI)
		m.AttachUI(uiBytes)
	}).Once()
	mock.On("generatedMessageHandler", msg).Once()
	handleGeneratedUIMessage(msg)
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
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		log = append(log, msg.(messages.MessageWithUI))
	}
	handleGeneratedUIMessage := makeGeneratedUIMessageHandler(assignUI, handleGeneratedMessage)

	wg := new(sync.WaitGroup)
	wg.Add(nrConcurrent)
	for i := 0; i < nrConcurrent; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < nrMessages; i++ {
				msg := mock_messages.NewMockMessageWithUI(ctrl)
				handleGeneratedUIMessage(msg)
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

func TestMakeGeneratedMessageConsumer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientID := rand.Uint32()

	log := mock_messagelog.NewMockMessageLog(ctrl)
	clientState := mock_clientstate.NewMockState(ctrl)
	clientStates := func(id uint32) clientstate.State {
		require.Equal(t, clientID, id)
		return clientState
	}

	consume := makeGeneratedMessageConsumer(log, clientStates)

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

func TestMakeGeneratedMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	applyReplicaMessage := func(msg messages.ReplicaMessage) error {
		args := mock.MethodCalled("replicaMessageApplier", msg)
		return args.Error(0)
	}
	consume := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageConsumer", msg)
	}
	handle := makeGeneratedMessageHandler(applyReplicaMessage, consume, logging.MustGetLogger(module))

	msg := mock_messages.NewMockReplicaMessage(ctrl)

	mock.On("replicaMessageApplier", msg).Return(fmt.Errorf("Error")).Once()
	assert.Panics(t, func() { handle(msg) }, "Failed to apply generated message")

	mock.On("replicaMessageApplier", msg).Return(nil).Once()
	mock.On("generatedMessageConsumer", msg).Once()
	handle(msg)
}
