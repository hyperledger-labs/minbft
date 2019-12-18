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
	"github.com/hyperledger-labs/minbft/messages"

	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
	mock_messagelog "github.com/hyperledger-labs/minbft/core/internal/messagelog/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/mocks"
)

func TestMakeIncomingMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	validateMessage := func(msg messages.Message) error {
		args := mock.MethodCalled("messageValidator", msg)
		return args.Error(0)
	}
	processMessage := func(msg messages.Message) (new bool, err error) {
		args := mock.MethodCalled("messageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	replyMessage := func(msg messages.Message) (reply <-chan messages.Message, err error) {
		args := mock.MethodCalled("messageReplier", msg)
		return args.Get(0).(chan messages.Message), args.Error(1)
	}
	handle := makeIncomingMessageHandler(validateMessage, processMessage, replyMessage)

	msg := struct {
		messages.Message
		i int
	}{i: rand.Int()}

	reply := struct {
		messages.Message
		i int
	}{i: rand.Int()}

	mock.On("messageValidator", msg).Return(fmt.Errorf("Error")).Once()
	_, _, err := handle(msg)
	assert.Error(t, err)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, fmt.Errorf("Error")).Once()
	_, _, err = handle(msg)
	assert.Error(t, err)

	nilRelyChan := (chan messages.Message)(nil)
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

	replyChan := make(chan messages.Message, 1)
	replyChan <- reply
	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	mock.On("messageReplier", msg).Return(replyChan, nil).Once()
	ch, new, err = handle(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Equal(t, reply, <-ch)

	replyChan = make(chan messages.Message, 1)
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	validateRequest := func(msg messages.Request) error {
		args := mock.MethodCalled("requestValidator", msg)
		return args.Error(0)
	}
	validatePrepare := func(msg messages.Prepare) error {
		args := mock.MethodCalled("prepareValidator", msg)
		return args.Error(0)
	}
	validateCommit := func(msg messages.Commit) error {
		args := mock.MethodCalled("commitValidator", msg)
		return args.Error(0)
	}
	validateMessage := makeMessageValidator(validateRequest, validatePrepare, validateCommit)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(0, 0, request)
	commit := messageImpl.NewCommit(0, prepare)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockMessage(ctrl)
		assert.Panics(t, func() { validateMessage(msg) }, "Unknown message type")
	})
	t.Run("Request", func(t *testing.T) {
		mock.On("requestValidator", request).Return(fmt.Errorf("Error")).Once()
		err := validateMessage(request)
		assert.Error(t, err, "Invalid Request")

		mock.On("requestValidator", request).Return(nil).Once()
		err = validateMessage(request)
		assert.NoError(t, err)
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("prepareValidator", prepare).Return(fmt.Errorf("Error")).Once()
		err := validateMessage(prepare)
		assert.Error(t, err, "Invalid Prepare")

		mock.On("prepareValidator", prepare).Return(nil).Once()
		err = validateMessage(prepare)
		assert.NoError(t, err)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("commitValidator", commit).Return(fmt.Errorf("Error")).Once()
		err := validateMessage(commit)
		assert.Error(t, err, "Invalid Commit")

		mock.On("commitValidator", commit).Return(nil).Once()
		err = validateMessage(commit)
		assert.NoError(t, err)
	})
}

func TestMakeMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	processRequest := func(msg messages.Request) (new bool, err error) {
		args := mock.MethodCalled("requestProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processReplicaMessage := func(msg messages.ReplicaMessage) (new bool, err error) {
		args := mock.MethodCalled("replicaMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makeMessageProcessor(processRequest, processReplicaMessage)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockMessage(ctrl)
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})
	t.Run("Request", func(t *testing.T) {
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
	})
	t.Run("ReplicaMessage", func(t *testing.T) {
		replicaMsg := struct {
			messages.ReplicaMessage
			i int
		}{i: rand.Int()}

		mock.On("replicaMessageProcessor", replicaMsg).Return(false, fmt.Errorf("")).Once()
		_, err := process(replicaMsg)
		assert.Error(t, err, "Failed to process replica message")

		mock.On("replicaMessageProcessor", replicaMsg).Return(false, nil).Once()
		new, err := process(replicaMsg)
		assert.NoError(t, err)
		assert.False(t, new)

		mock.On("replicaMessageProcessor", replicaMsg).Return(true, nil).Once()
		new, err = process(replicaMsg)
		assert.NoError(t, err)
		assert.True(t, new)
	})
}

func TestMakeReplicaMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n := randN()
	id := randReplicaID(n)

	processMessage := func(msg messages.Message) (new bool, err error) {
		args := mock.MethodCalled("messageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processUIMessage := func(msg messages.CertifiedMessage) (new bool, err error) {
		args := mock.MethodCalled("uiMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makeReplicaMessageProcessor(id, processMessage, processUIMessage)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockReplicaMessage(ctrl)
		msg.EXPECT().ReplicaID().Return(randOtherReplicaID(id, n)).AnyTimes()
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})
	t.Run("Prepare", func(t *testing.T) {
		ownPrepare := messageImpl.NewPrepare(id, viewForPrimary(n, id), request)
		new, err := process(ownPrepare)
		assert.NoError(t, err)
		assert.False(t, new, "Own Prepare")

		primary := randOtherReplicaID(id, n)
		prepare := messageImpl.NewPrepare(primary, viewForPrimary(n, primary), request)

		mock.On("messageProcessor", request).Return(false, fmt.Errorf("Error")).Once()
		_, err = process(prepare)
		assert.Error(t, err, "Failed to process embedded Request")

		mock.On("messageProcessor", request).Return(true, nil).Once()
		mock.On("uiMessageProcessor", prepare).Return(false, fmt.Errorf("Error")).Once()
		_, err = process(prepare)
		assert.Error(t, err, "Failed to finish processing Prepare")

		mock.On("messageProcessor", request).Return(true, nil).Once()
		mock.On("uiMessageProcessor", prepare).Return(true, nil).Once()
		new, err = process(prepare)
		assert.NoError(t, err)
		assert.True(t, new)

		mock.On("messageProcessor", request).Return(true, nil).Once()
		mock.On("uiMessageProcessor", prepare).Return(false, nil).Once()
		new, err = process(prepare)
		assert.NoError(t, err)
		assert.False(t, new)
	})
	t.Run("Commit", func(t *testing.T) {
		primary := randOtherReplicaID(id, n)
		prepare := messageImpl.NewPrepare(primary, viewForPrimary(n, primary), request)

		ownCommit := messageImpl.NewCommit(id, prepare)
		new, err := process(ownCommit)
		assert.NoError(t, err)
		assert.False(t, new, "Own Commit")

		commit := messageImpl.NewCommit(randOtherReplicaID(id, n), prepare)

		mock.On("messageProcessor", prepare).Return(false, fmt.Errorf("Error")).Once()
		_, err = process(commit)
		assert.Error(t, err, "Failed to process embedded Prepare")

		mock.On("messageProcessor", prepare).Return(true, nil).Once()
		mock.On("uiMessageProcessor", commit).Return(false, fmt.Errorf("Error")).Once()
		_, err = process(commit)
		assert.Error(t, err, "Failed to finish processing Commit")

		mock.On("messageProcessor", prepare).Return(true, nil).Once()
		mock.On("uiMessageProcessor", commit).Return(true, nil).Once()
		new, err = process(commit)
		assert.NoError(t, err)
		assert.True(t, new)

		mock.On("messageProcessor", prepare).Return(true, nil).Once()
		mock.On("uiMessageProcessor", commit).Return(false, nil).Once()
		new, err = process(commit)
		assert.NoError(t, err)
		assert.False(t, new)
	})
}

func TestMakeUIMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	captureUI := func(msg messages.CertifiedMessage) (new bool, release func()) {
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

	uiMsg := struct {
		messages.CertifiedMessage
		i int
	}{i: rand.Int()}

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

	n := randN()
	primary := randReplicaID(n)
	view := viewForPrimary(n, primary)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(primary, view, request)
	commit := messageImpl.NewCommit(randOtherReplicaID(primary, n), prepare)

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

	applyPrepare := func(msg messages.Prepare) error {
		args := mock.MethodCalled("prepareApplier", msg)
		return args.Error(0)
	}
	applyCommit := func(msg messages.Commit) error {
		args := mock.MethodCalled("commitApplier", msg)
		return args.Error(0)
	}
	apply := makeReplicaMessageApplier(applyPrepare, applyCommit)

	reqSeq := rand.Uint64()
	request := messageImpl.NewRequest(0, reqSeq, nil)
	prepare := messageImpl.NewPrepare(0, 0, request)
	commit := messageImpl.NewCommit(1, prepare)
	reply := messageImpl.NewReply(1, 0, reqSeq, nil)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockReplicaMessage(ctrl)
		assert.Panics(t, func() { apply(msg) }, "Unknown message type")
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("prepareApplier", prepare).Return(fmt.Errorf("Error")).Once()
		err := apply(prepare)
		assert.Error(t, err, "Failed to apply Prepare")

		mock.On("prepareApplier", prepare).Return(nil).Once()
		err = apply(prepare)
		assert.NoError(t, err)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("commitApplier", commit).Return(fmt.Errorf("Error")).Once()
		err := apply(commit)
		assert.Error(t, err, "Failed to apply Commit")

		mock.On("commitApplier", commit).Return(nil).Once()
		err = apply(commit)
		assert.NoError(t, err)
	})
	t.Run("Reply", func(t *testing.T) {
		err := apply(reply)
		assert.NoError(t, err)
	})
}

func TestMakeMessageReplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	replyRequest := func(request messages.Request) <-chan messages.Reply {
		args := mock.MethodCalled("requestReplier", request)
		return args.Get(0).(chan messages.Reply)
	}

	replyMessage := makeMessageReplier(replyRequest)

	seq := rand.Uint64()
	request := messageImpl.NewRequest(0, seq, nil)
	prepare := messageImpl.NewPrepare(0, 0, request)
	commit := messageImpl.NewCommit(1, prepare)
	reply := messageImpl.NewReply(1, 0, seq, nil)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockMessage(ctrl)
		assert.Panics(t, func() { replyMessage(msg) }, "Unknown message type")
	})
	t.Run("Request", func(t *testing.T) {
		replyChan := make(chan messages.Reply, 1)
		replyChan <- reply
		mock.On("requestReplier", request).Return(replyChan).Once()
		ch, err := replyMessage(request)
		assert.NoError(t, err)
		assert.Equal(t, reply, <-ch)
	})
	t.Run("Prepare", func(t *testing.T) {
		ch, err := replyMessage(prepare)
		assert.NoError(t, err)
		assert.Nil(t, ch)
	})
	t.Run("Commit", func(t *testing.T) {
		ch, err := replyMessage(commit)
		assert.NoError(t, err)
		assert.Nil(t, ch)
	})
}

func TestMakeGeneratedUIMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	assignUI := func(msg messages.CertifiedMessage) {
		mock.MethodCalled("uiAssigner", msg)
	}
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	handleGeneratedUIMessage := makeGeneratedUIMessageHandler(assignUI, handleGeneratedMessage)

	msg := struct {
		messages.CertifiedMessage
		i int
	}{i: rand.Int()}

	mock.On("uiAssigner", msg).Once()
	mock.On("generatedMessageHandler", msg).Once()
	handleGeneratedUIMessage(msg)
}

func TestMakeGeneratedUIMessageHandlerConcurrent(t *testing.T) {
	const nrMessages = 10
	const nrConcurrent = 5

	type uiMsg struct {
		messages.CertifiedMessage
		cv uint64
	}

	cv := uint64(0)
	log := make([]*uiMsg, 0, nrMessages*nrConcurrent)

	assignUI := func(msg messages.CertifiedMessage) {
		cv++
		msg.(*uiMsg).cv = cv
	}
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		log = append(log, msg.(*uiMsg))
	}
	handleGeneratedUIMessage := makeGeneratedUIMessageHandler(assignUI, handleGeneratedMessage)

	wg := new(sync.WaitGroup)
	wg.Add(nrConcurrent)
	for i := 0; i < nrConcurrent; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < nrMessages; i++ {
				handleGeneratedUIMessage(&uiMsg{})
			}
		}()
	}
	wg.Wait()

	assert.Len(t, log, nrMessages*nrConcurrent)
	for i, m := range log {
		assert.EqualValues(t, uint64(i+1), m.cv)
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

	t.Run("Reply", func(t *testing.T) {
		reply := messageImpl.NewReply(rand.Uint32(), clientID, rand.Uint64(), nil)

		clientState.EXPECT().AddReply(reply).Return(nil)
		consume(reply)

		clientState.EXPECT().AddReply(reply).Return(fmt.Errorf("Invalid request ID"))
		assert.Panics(t, func() { consume(reply) })
	})
	t.Run("ReplicaMessage", func(t *testing.T) {
		msg := struct {
			messages.ReplicaMessage
			i int
		}{i: rand.Int()}

		log.EXPECT().Append(msg)
		consume(msg)
	})
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
