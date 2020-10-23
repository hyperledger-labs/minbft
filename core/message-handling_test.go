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

	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/common/logger"
	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/messages"

	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
	mock_messagelog "github.com/hyperledger-labs/minbft/core/internal/messagelog/mocks"
	mock_viewstate "github.com/hyperledger-labs/minbft/core/internal/viewstate/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/mocks"
)

func TestMakeOwnMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	processMessage := func(msg messages.Message) (new bool, err error) {
		args := mock.MethodCalled("messageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	handle := makeOwnMessageHandler(processMessage)

	msg := struct {
		messages.Message
		i int
	}{i: rand.Int()}

	mock.On("messageProcessor", msg).Return(false, fmt.Errorf("error")).Once()
	_, _, err := handle(msg)
	assert.Error(t, err)

	mock.On("messageProcessor", msg).Return(false, nil).Once()
	ch, new, err := handle(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Nil(t, ch)

	mock.On("messageProcessor", msg).Return(true, nil).Once()
	ch, new, err = handle(msg)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.Nil(t, ch)
}

func TestMakePeerMessageHandler(t *testing.T) {
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
	handle := makePeerMessageHandler(validateMessage, processMessage)

	msg := struct {
		messages.Message
		i int
	}{i: rand.Int()}

	mock.On("messageValidator", msg).Return(fmt.Errorf("error")).Once()
	_, _, err := handle(msg)
	assert.Error(t, err)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, fmt.Errorf("error")).Once()
	_, _, err = handle(msg)
	assert.Error(t, err)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	ch, new, err := handle(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Nil(t, ch)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(true, nil).Once()
	ch, new, err = handle(msg)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.Nil(t, ch)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	ch, new, err = handle(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Nil(t, ch)
}

func TestMakeClientMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	validateRequest := func(msg messages.Request) error {
		args := mock.MethodCalled("requestValidator", msg)
		return args.Error(0)
	}
	processRequest := func(msg messages.Request) (new bool, err error) {
		args := mock.MethodCalled("requestProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	replyRequest := func(request messages.Request) <-chan messages.Reply {
		args := mock.MethodCalled("requestReplier", request)
		return args.Get(0).(chan messages.Reply)
	}

	handle := makeClientMessageHandler(validateRequest, processRequest, replyRequest)

	msg := struct{ messages.Message }{}

	cl := rand.Uint32()
	seq := rand.Uint64()
	req := messageImpl.NewRequest(cl, seq, nil)
	reply := messageImpl.NewReply(rand.Uint32(), cl, seq, nil)

	_, _, err := handle(msg)
	assert.Error(t, err, "Unexpected message")

	mock.On("requestValidator", req).Return(fmt.Errorf("error")).Once()
	_, _, err = handle(req)
	assert.Error(t, err)

	mock.On("requestValidator", req).Return(nil).Once()
	mock.On("requestProcessor", req).Return(false, fmt.Errorf("error")).Once()
	_, _, err = handle(req)
	assert.Error(t, err)

	replyChan := make(chan messages.Reply, 1)
	replyChan <- reply
	mock.On("requestValidator", req).Return(nil).Once()
	mock.On("requestProcessor", req).Return(false, nil).Once()
	mock.On("requestReplier", req).Return(replyChan).Once()
	ch, new, err := handle(req)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.EqualValues(t, reply, <-ch)

	replyChan = make(chan messages.Reply, 1)
	replyChan <- reply
	mock.On("requestValidator", req).Return(nil).Once()
	mock.On("requestProcessor", req).Return(true, nil).Once()
	mock.On("requestReplier", req).Return(replyChan).Once()
	ch, new, err = handle(req)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.EqualValues(t, reply, <-ch)
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
		mock.On("requestValidator", request).Return(fmt.Errorf("error")).Once()
		err := validateMessage(request)
		assert.Error(t, err, "Invalid Request")

		mock.On("requestValidator", request).Return(nil).Once()
		err = validateMessage(request)
		assert.NoError(t, err)
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("prepareValidator", prepare).Return(fmt.Errorf("error")).Once()
		err := validateMessage(prepare)
		assert.Error(t, err, "Invalid Prepare")

		mock.On("prepareValidator", prepare).Return(nil).Once()
		err = validateMessage(prepare)
		assert.NoError(t, err)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("commitValidator", commit).Return(fmt.Errorf("error")).Once()
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
	processPeerMessage := func(msg messages.PeerMessage) (new bool, err error) {
		args := mock.MethodCalled("peerMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makeMessageProcessor(processRequest, processPeerMessage)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockMessage(ctrl)
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})
	t.Run("Request", func(t *testing.T) {
		mock.On("requestProcessor", request).Return(false, fmt.Errorf("error")).Once()
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
	t.Run("PeerMessage", func(t *testing.T) {
		peerMsg := struct {
			messages.PeerMessage
			i int
		}{i: rand.Int()}

		mock.On("peerMessageProcessor", peerMsg).Return(false, fmt.Errorf("")).Once()
		_, err := process(peerMsg)
		assert.Error(t, err, "Failed to process replica message")

		mock.On("peerMessageProcessor", peerMsg).Return(false, nil).Once()
		new, err := process(peerMsg)
		assert.NoError(t, err)
		assert.False(t, new)

		mock.On("peerMessageProcessor", peerMsg).Return(true, nil).Once()
		new, err = process(peerMsg)
		assert.NoError(t, err)
		assert.True(t, new)
	})
}

func TestMakePeerMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	processEmbedded := func(msg messages.PeerMessage) {
		mock.MethodCalled("embeddedMessageProcessor", msg)
	}
	processUIMessage := func(msg messages.CertifiedMessage) (new bool, err error) {
		args := mock.MethodCalled("uiMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makePeerMessageProcessor(processEmbedded, processUIMessage)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})
	t.Run("CertifiedMessage", func(t *testing.T) {
		type certifiedPeerMessage interface {
			messages.CertifiedMessage
			ImplementsPeerMessage()
		}
		msg := struct {
			certifiedPeerMessage
			v int
		}{v: rand.Int()}

		mock.On("embeddedMessageProcessor", msg).Once()
		mock.On("uiMessageProcessor", msg).Return(true, nil).Once()
		_, err := process(msg)
		assert.NoError(t, err)

		mock.On("embeddedMessageProcessor", msg).Once()
		mock.On("uiMessageProcessor", msg).Return(false, fmt.Errorf("error")).Once()
		_, err = process(msg)
		assert.Error(t, err, "Failed to finish processing certified message")

		mock.On("embeddedMessageProcessor", msg).Once()
		mock.On("uiMessageProcessor", msg).Return(true, nil).Once()
		new, err := process(msg)
		assert.NoError(t, err)
		assert.True(t, new)

		mock.On("embeddedMessageProcessor", msg).Once()
		mock.On("uiMessageProcessor", msg).Return(false, nil).Once()
		new, err = process(msg)
		assert.NoError(t, err)
		assert.False(t, new)

	})
}

func TestMakeEmbeddedMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	processMessage := func(msg messages.Message) (new bool, err error) {
		args := mock.MethodCalled("messageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}

	process := makeEmbeddedMessageProcessor(processMessage, logger.NewReplicaLogger(0))

	n, view := randN(), randView()
	primary := primaryID(n, view)
	backup := randOtherReplicaID(primary, n)
	request := messageImpl.NewRequest(rand.Uint32(), rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(primary, view, request)
	commit := messageImpl.NewCommit(backup, prepare)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("messageProcessor", request).Return(false, nil).Once()
		process(prepare)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("messageProcessor", prepare).Return(false, nil).Once()
		process(commit)
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
	processViewMessage := func(msg messages.PeerMessage) (new bool, err error) {
		args := mock.MethodCalled("viewMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makeUIMessageProcessor(captureUI, processViewMessage)

	type certifiedPeerMessage interface {
		messages.CertifiedMessage
		ImplementsPeerMessage()
	}
	uiMsg := struct {
		certifiedPeerMessage
		i int
	}{i: rand.Int()}

	mock.On("uiCapturer", uiMsg).Return(false).Once()
	new, err := process(uiMsg)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("uiCapturer", uiMsg).Return(true).Once()
	mock.On("viewMessageProcessor", uiMsg).Return(false, fmt.Errorf("error")).Once()
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

	viewState := mock_viewstate.NewMockState(ctrl)
	applyPeerMessage := func(msg messages.PeerMessage, active bool) error {
		args := mock.MethodCalled("peerMessageApplier", msg, active)
		return args.Error(0)
	}
	process := makeViewMessageProcessor(viewState, applyPeerMessage)

	n := randN()
	primary := randReplicaID(n)
	view := viewForPrimary(n, primary) + uint64(n)
	oldView := view - uint64(1+rand.Intn(int(n-1)))
	newView := view + uint64(1+rand.Intn(int(n-1)))

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(primary, view, request)
	commit := messageImpl.NewCommit(randOtherReplicaID(primary, n), prepare)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})

	testPeerMessage := func(t *testing.T, msg messages.PeerMessage) {
		viewState.EXPECT().HoldView().Return(view, newView, func() {
			mock.MethodCalled("viewReleaser")
		})
		mock.On("viewReleaser").Once()
		mock.On("peerMessageApplier", msg, false).Return(nil).Once()
		new, err := process(msg)
		assert.NoError(t, err)
		assert.True(t, new)

		viewState.EXPECT().HoldView().Return(newView, newView, func() {
			mock.MethodCalled("viewReleaser")
		})
		mock.On("viewReleaser").Once()
		new, err = process(msg)
		assert.NoError(t, err)
		assert.False(t, new, "Message for former view")

		viewState.EXPECT().HoldView().Return(oldView, oldView, func() {
			mock.MethodCalled("viewReleaser")
		})
		mock.On("viewReleaser").Once()
		_, err = process(msg)
		assert.Error(t, err, "Message for unexpected view")

		viewState.EXPECT().HoldView().Return(view, view, func() {
			mock.MethodCalled("viewReleaser")
		})
		mock.On("peerMessageApplier", msg, true).Return(nil).Once()
		mock.On("viewReleaser").Once()
		new, err = process(msg)
		assert.NoError(t, err)
		assert.True(t, new)
	}
	t.Run("Prepare", func(t *testing.T) {
		testPeerMessage(t, prepare)
	})
	t.Run("Commit", func(t *testing.T) {
		testPeerMessage(t, commit)
	})
}

func TestMakePeerMessageApplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	applyPrepare := func(msg messages.Prepare, active bool) error {
		args := mock.MethodCalled("prepareApplier", msg, active)
		return args.Error(0)
	}
	applyCommit := func(msg messages.Commit, active bool) error {
		args := mock.MethodCalled("commitApplier", msg, active)
		return args.Error(0)
	}
	apply := makePeerMessageApplier(applyPrepare, applyCommit)

	reqSeq := rand.Uint64()
	request := messageImpl.NewRequest(0, reqSeq, nil)
	prepare := messageImpl.NewPrepare(0, 0, request)
	commit := messageImpl.NewCommit(1, prepare)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		assert.Panics(t, func() { apply(msg, true) }, "Unknown message type")
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("prepareApplier", prepare, true).Return(fmt.Errorf("error")).Once()
		err := apply(prepare, true)
		assert.Error(t, err, "Failed to apply Prepare")

		mock.On("prepareApplier", prepare, true).Return(nil).Once()
		err = apply(prepare, true)
		assert.NoError(t, err)

		mock.On("prepareApplier", prepare, false).Return(nil).Once()
		err = apply(prepare, false)
		assert.NoError(t, err)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("commitApplier", commit, true).Return(fmt.Errorf("error")).Once()
		err := apply(commit, true)
		assert.Error(t, err, "Failed to apply Commit")

		mock.On("commitApplier", commit, true).Return(nil).Once()
		err = apply(commit, true)
		assert.NoError(t, err)

		mock.On("commitApplier", commit, false).Return(nil).Once()
		err = apply(commit, false)
		assert.NoError(t, err)
	})
}

func TestMakeGeneratedMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sign := func(msg messages.SignedMessage) {
		mock.MethodCalled("messageSigner", msg)
	}
	assignUI := func(msg messages.CertifiedMessage) {
		mock.MethodCalled("uiAssigner", msg)
	}
	consumeGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageConsumer", msg)
	}
	handle := makeGeneratedMessageHandler(sign, assignUI, consumeGeneratedMessage)

	certifiedMsg := struct {
		messages.CertifiedMessage
		i int
	}{i: rand.Int()}

	signedMsg := struct {
		messages.SignedMessage
		messages.ReplicaMessage
		i int
	}{i: rand.Int()}

	mock.On("uiAssigner", certifiedMsg).Once()
	mock.On("generatedMessageConsumer", certifiedMsg).Once()
	handle(certifiedMsg)

	mock.On("messageSigner", signedMsg).Once()
	mock.On("generatedMessageConsumer", signedMsg).Once()
	handle(signedMsg)
}

func TestMakeGeneratedMessageHandlerConcurrent(t *testing.T) {
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
	handle := makeGeneratedMessageHandler(nil, assignUI, handleGeneratedMessage)

	wg := new(sync.WaitGroup)
	wg.Add(nrConcurrent)
	for i := 0; i < nrConcurrent; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < nrMessages; i++ {
				handle(&uiMsg{})
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

	consume := makeGeneratedMessageConsumer(log, clientStates, logger.NewReplicaLogger(0))

	t.Run("Reply", func(t *testing.T) {
		reply := messageImpl.NewReply(rand.Uint32(), clientID, rand.Uint64(), nil)

		clientState.EXPECT().AddReply(reply).Return(nil)
		consume(reply)

		clientState.EXPECT().AddReply(reply).Return(fmt.Errorf("invalid request ID"))
		assert.Panics(t, func() { consume(reply) })
	})
	t.Run("PeerMessage", func(t *testing.T) {
		msg := struct {
			messages.ReplicaMessage
			i int
		}{i: rand.Int()}

		log.EXPECT().Append(msg)
		consume(msg)
	})
}
