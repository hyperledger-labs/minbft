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

	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/common/logger"
	"github.com/hyperledger-labs/minbft/messages"

	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
	mock_messagelog "github.com/hyperledger-labs/minbft/core/internal/messagelog/mocks"
	mock_viewstate "github.com/hyperledger-labs/minbft/core/internal/viewstate/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/mocks"
	. "github.com/hyperledger-labs/minbft/messages/testing"
)

func TestMakeOwnMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	processMessage := func(msg messages.Message) (new bool, err error) {
		args := mock.MethodCalled("messageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	handleEmbedded := func(msg messages.Message) error {
		args := mock.MethodCalled("embeddedMessageHandler", msg)
		return args.Error(0)
	}
	handle := makeOwnMessageHandler(handleEmbedded, processMessage)

	msg := struct {
		messages.Message
		i int
	}{i: rand.Int()}

	mock.On("embeddedMessageHandler", msg).Return(fmt.Errorf("error")).Once()
	_, _, err := handle(msg)
	assert.Error(t, err)

	mock.On("embeddedMessageHandler", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, fmt.Errorf("error")).Once()
	_, _, err = handle(msg)
	assert.Error(t, err)

	mock.On("embeddedMessageHandler", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	ch, new, err := handle(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Nil(t, ch)

	mock.On("embeddedMessageHandler", msg).Return(nil).Once()
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
	handleEmbedded := func(msg messages.Message) error {
		args := mock.MethodCalled("embeddedMessageHandler", msg)
		return args.Error(0)
	}
	processMessage := func(msg messages.Message) (new bool, err error) {
		args := mock.MethodCalled("messageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	handle := makePeerMessageHandler(validateMessage, handleEmbedded, processMessage)

	msg := struct {
		messages.Message
		i int
	}{i: rand.Int()}

	mock.On("messageValidator", msg).Return(fmt.Errorf("error")).Once()
	_, _, err := handle(msg)
	assert.Error(t, err)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("embeddedMessageHandler", msg).Return(fmt.Errorf("error")).Once()
	_, _, err = handle(msg)
	assert.Error(t, err)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("embeddedMessageHandler", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, fmt.Errorf("error")).Once()
	_, _, err = handle(msg)
	assert.Error(t, err)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("embeddedMessageHandler", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(false, nil).Once()
	ch, new, err := handle(msg)
	assert.NoError(t, err)
	assert.False(t, new)
	assert.Nil(t, ch)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("embeddedMessageHandler", msg).Return(nil).Once()
	mock.On("messageProcessor", msg).Return(true, nil).Once()
	ch, new, err = handle(msg)
	assert.NoError(t, err)
	assert.True(t, new)
	assert.Nil(t, ch)

	mock.On("messageValidator", msg).Return(nil).Once()
	mock.On("embeddedMessageHandler", msg).Return(nil).Once()
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
	validatePeerMessage := func(msg messages.PeerMessage) error {
		args := mock.MethodCalled("peerMessageValidator", msg)
		return args.Error(0)
	}
	validateMessage := makeMessageValidator(validateRequest, validatePeerMessage)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockMessage(ctrl)
		assert.Panics(t, func() { validateMessage(msg) }, "Unknown message type")
	})
	t.Run("Request", func(t *testing.T) {
		req := messageImpl.NewRequest(rand.Uint32(), rand.Uint64(), randBytes())

		mock.On("requestValidator", req).Return(fmt.Errorf("error")).Once()
		err := validateMessage(req)
		assert.Error(t, err, "Invalid Request")

		mock.On("requestValidator", req).Return(nil).Once()
		err = validateMessage(req)
		assert.NoError(t, err)
	})
	t.Run("PeerMessage", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		msg.EXPECT().ReplicaID().Return(randReplicaID(randN())).AnyTimes()

		mock.On("peerMessageValidator", msg).Return(fmt.Errorf("error")).Once()
		err := validateMessage(msg)
		assert.Error(t, err, "Invalid peer message")

		mock.On("peerMessageValidator", msg).Return(nil).Once()
		err = validateMessage(msg)
		assert.NoError(t, err)
	})
}

func TestMakeCertifiedMessageValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, v := randN(), randView()
	p := primaryID(n, v)

	validatePrepare := func(msg messages.Prepare) error {
		args := mock.MethodCalled("prepareValidator", msg)
		return args.Error(0)
	}
	validateCommit := func(msg messages.Commit) error {
		args := mock.MethodCalled("commitValidator", msg)
		return args.Error(0)
	}
	verifyUI := func(msg messages.CertifiedMessage) error {
		args := mock.MethodCalled("uiVerifier", msg)
		return args.Error(0)
	}
	validateViewChange := func(msg messages.ViewChange) error {
		args := mock.MethodCalled("viewChangeValidator", msg)
		return args.Error(0)
	}
	validateNewView := func(msg messages.NewView) error {
		args := mock.MethodCalled("newViewValidator", msg)
		return args.Error(0)
	}
	validate := makeCertifiedMessageValidator(validatePrepare, validateCommit, validateViewChange, validateNewView, verifyUI)

	request := messageImpl.NewRequest(rand.Uint32(), rand.Uint64(), randBytes())
	prepare := messageImpl.NewPrepare(p, v, request)
	commit := messageImpl.NewCommit(randOtherReplicaID(p, n), prepare)
	vc := messageImpl.NewViewChange(randReplicaID(n), randView()+1, nil, nil)
	nv := messageImpl.NewNewView(randReplicaID(n), randView()+1, nil)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockCertifiedMessage(ctrl)
		assert.Panics(t, func() { validate(msg) }, "Unknown message type")
	})
	t.Run("InvalidUI", func(t *testing.T) {
		msg := mock_messages.NewMockCertifiedMessage(ctrl)
		mock.On("uiVerifier", msg).Return(fmt.Errorf("error")).Once()
		err := validate(msg)
		assert.Error(t, err, "Invalid UI")
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("uiVerifier", prepare).Return(nil).Once()
		mock.On("prepareValidator", prepare).Return(fmt.Errorf("error")).Once()
		err := validate(prepare)
		assert.Error(t, err, "Invalid Prepare")

		mock.On("uiVerifier", prepare).Return(nil).Once()
		mock.On("prepareValidator", prepare).Return(nil).Once()
		err = validate(prepare)
		assert.NoError(t, err)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("uiVerifier", commit).Return(nil).Once()
		mock.On("commitValidator", commit).Return(fmt.Errorf("error")).Once()
		err := validate(commit)
		assert.Error(t, err, "Invalid Commit")

		mock.On("uiVerifier", commit).Return(nil).Once()
		mock.On("commitValidator", commit).Return(nil).Once()
		err = validate(commit)
		assert.NoError(t, err)
	})
	t.Run("ViewChange", func(t *testing.T) {
		mock.On("uiVerifier", vc).Return(nil).Once()
		mock.On("viewChangeValidator", vc).Return(fmt.Errorf("error")).Once()
		err := validate(vc)
		assert.Error(t, err, "Invalid ViewChange")

		mock.On("uiVerifier", vc).Return(nil).Once()
		mock.On("viewChangeValidator", vc).Return(nil).Once()
		err = validate(vc)
		assert.NoError(t, err)
	})
	t.Run("NewView", func(t *testing.T) {
		mock.On("uiVerifier", nv).Return(nil).Once()
		mock.On("newViewValidator", nv).Return(fmt.Errorf("Error")).Once()
		err := validate(nv)
		assert.Error(t, err, "Invalid NewView")

		mock.On("uiVerifier", nv).Return(nil).Once()
		mock.On("newViewValidator", nv).Return(nil).Once()
		err = validate(nv)
		assert.NoError(t, err)
	})
}

func TestMakePeerMessageValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n, v := randN(), randView()

	validateCertified := func(msg messages.CertifiedMessage) error {
		args := mock.MethodCalled("certifiedMessageValidator", msg)
		return args.Error(0)
	}
	validateReqViewChange := func(msg messages.ReqViewChange) error {
		args := mock.MethodCalled("reqViewChangeValidator", msg)
		return args.Error(0)
	}
	validatePeerMessage := makePeerMessageValidator(n, validateCertified, validateReqViewChange)

	rvc := messageImpl.NewReqViewChange(randReplicaID(n), v+1)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		msg.EXPECT().ReplicaID().Return(randReplicaID(n)).AnyTimes()
		assert.Panics(t, func() { validatePeerMessage(msg) }, "Unknown message type")
	})
	t.Run("InvalidReplicaID", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		msg.EXPECT().ReplicaID().Return(n + uint32(rand.Intn(2))).AnyTimes()
		err := validatePeerMessage(msg)
		assert.Error(t, err, "Invalid replica ID")
	})
	t.Run("CertifiedMessage", func(t *testing.T) {
		certifiedMsg := mock_messages.NewMockCertifiedMessage(ctrl)
		certifiedMsg.EXPECT().ReplicaID().Return(randReplicaID(n)).AnyTimes()
		type peerMessage interface {
			ImplementsPeerMessage()
		}
		msg := struct {
			messages.CertifiedMessage
			peerMessage
		}{CertifiedMessage: certifiedMsg}

		mock.On("certifiedMessageValidator", msg).Return(fmt.Errorf("error")).Once()
		err := validatePeerMessage(msg)
		assert.Error(t, err, "Invalid message")

		mock.On("certifiedMessageValidator", msg).Return(nil).Once()
		err = validatePeerMessage(msg)
		assert.NoError(t, err)
	})
	t.Run("ReqViewChange", func(t *testing.T) {
		mock.On("reqViewChangeValidator", rvc).Return(fmt.Errorf("Error")).Once()
		err := validatePeerMessage(rvc)
		assert.Error(t, err, "Invalid ReqViewChange")

		mock.On("reqViewChangeValidator", rvc).Return(nil).Once()
		err = validatePeerMessage(rvc)
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

	n := randN()

	processCertifiedMessage := func(msg messages.CertifiedMessage) (new bool, err error) {
		args := mock.MethodCalled("certifiedMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processReqViewChange := func(msg messages.ReqViewChange) (new bool, err error) {
		args := mock.MethodCalled("reqViewChangeProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	process := makePeerMessageProcessor(n, processCertifiedMessage, processReqViewChange)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		msg.EXPECT().ReplicaID().Return(randReplicaID(n)).AnyTimes()
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})
	t.Run("CertifiedMessage", func(t *testing.T) {
		certifiedMsg := mock_messages.NewMockCertifiedMessage(ctrl)
		certifiedMsg.EXPECT().ReplicaID().Return(randReplicaID(n)).AnyTimes()
		type peerMessage interface {
			ImplementsPeerMessage()
		}
		msg := struct {
			messages.CertifiedMessage
			peerMessage
		}{CertifiedMessage: certifiedMsg}

		mock.On("certifiedMessageProcessor", msg).Return(false, fmt.Errorf("error")).Once()
		_, err := process(msg)
		assert.Error(t, err, "Failed to finish processing certified message")

		mock.On("certifiedMessageProcessor", msg).Return(true, nil).Once()
		new, err := process(msg)
		assert.NoError(t, err)
		assert.True(t, new)

		mock.On("certifiedMessageProcessor", msg).Return(false, nil).Once()
		new, err = process(msg)
		assert.NoError(t, err)
		assert.False(t, new)
	})
	t.Run("ReqViewChange", func(t *testing.T) {
		msg := messageImpl.NewReqViewChange(randReplicaID(n), rand.Uint64())

		mock.On("reqViewChangeProcessor", msg).Return(false, fmt.Errorf("error")).Once()
		_, err := process(msg)
		assert.Error(t, err, "Failed to finish processing certified message")

		mock.On("reqViewChangeProcessor", msg).Return(true, nil).Once()
		new, err := process(msg)
		assert.NoError(t, err)
		assert.True(t, new)

		mock.On("reqViewChangeProcessor", msg).Return(false, nil).Once()
		new, err = process(msg)
		assert.NoError(t, err)
		assert.False(t, new)
	})
}

func TestMakeEmbeddedMessageHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handleMessage := func(msg messages.Message) (_ <-chan messages.Message, new bool, err error) {
		args := mock.MethodCalled("messageHandler", msg)
		return nil, args.Bool(0), args.Error(1)
	}

	handle := makeEmbeddedMessageHandler(handleMessage)

	n, view := randN(), randView()
	primary := primaryID(n, view)
	backup := randOtherReplicaID(primary, n)
	request := messageImpl.NewRequest(rand.Uint32(), rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(primary, view, request)
	commit := messageImpl.NewCommit(backup, prepare)
	prepare.SetUI(testUI(rand.Uint64()))
	commit.SetUI(testUI(rand.Uint64()))

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		assert.Panics(t, func() { handle(msg) }, "Unknown message type")
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("messageHandler", request).Return(false, fmt.Errorf("error")).Once()
		err := handle(prepare)
		assert.Error(t, err)

		mock.On("messageHandler", request).Return(false, nil).Once()
		err = handle(prepare)
		assert.NoError(t, err)

		mock.On("messageHandler", request).Return(true, nil).Once()
		err = handle(prepare)
		assert.NoError(t, err)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("messageHandler", prepare).Return(false, fmt.Errorf("error")).Once()
		err := handle(commit)
		assert.Error(t, err)

		mock.On("messageHandler", prepare).Return(false, nil).Once()
		err = handle(commit)
		assert.NoError(t, err)

		mock.On("messageHandler", prepare).Return(true, nil).Once()
		err = handle(commit)
		assert.NoError(t, err)
	})
	t.Run("ViewChange", func(t *testing.T) {
		reqs := []messages.Request{
			MakeTestReq(messageImpl, rand.Uint32(), rand.Uint64(), randBytes()),
			MakeTestReq(messageImpl, rand.Uint32(), rand.Uint64(), randBytes()),
		}
		log := messages.MessageLog{
			MakeTestComm(messageImpl, 1, MakeTestPrep(messageImpl, 0, 0, reqs[0], 1), 1),
			MakeTestComm(messageImpl, 1, MakeTestPrep(messageImpl, 0, 0, reqs[1], 2), 2),
		}
		vcCert := RandVCCert(messageImpl, 1, 3, 1)
		vc := MakeTestVC(messageImpl, 1, 1, log, vcCert, 3)

		for _, m := range vcCert {
			mock.On("messageHandler", m).Return(true, nil).Once()
		}
		for _, m := range log {
			mock.On("messageHandler", m).Return(true, nil).Once()
		}
		err := handle(vc)
		assert.NoError(t, err)

		for _, m := range vcCert {
			mock.On("messageHandler", m).Return(false, nil).Once()
		}
		for _, m := range log {
			mock.On("messageHandler", m).Return(false, nil).Once()
		}
		err = handle(vc)
		assert.NoError(t, err)

		mock.On("messageHandler", vcCert[rand.Intn(len(vcCert))]).Return(false, fmt.Errorf("error")).Once()
		mock.On("messageHandler", log[rand.Intn(len(log))]).Return(false, fmt.Errorf("error")).Once()

		for _, m := range vcCert {
			mock.On("messageHandler", m).Return(false, nil)
		}
		for _, m := range log {
			mock.On("messageHandler", m).Return(false, nil)
		}

		err = handle(vc)
		assert.Error(t, err)

		err = handle(vc)
		assert.Error(t, err)

		err = handle(vc)
		assert.NoError(t, err)
	})
	t.Run("NewView", func(t *testing.T) {
		nvCert := MakeTestNVCert(messageImpl)
		nv := messageImpl.NewNewView(1, 1, nvCert)

		for _, m := range nvCert {
			mock.On("messageHandler", m).Return(true, nil).Once()
		}
		err := handle(nv)
		assert.NoError(t, err)

		for _, m := range nvCert {
			mock.On("messageHandler", m).Return(false, nil).Once()
		}
		err = handle(nv)
		assert.NoError(t, err)

		mock.On("messageHandler", nvCert[rand.Intn(len(nvCert))]).Return(false, fmt.Errorf("error")).Once()

		for _, m := range nvCert {
			mock.On("messageHandler", m).Return(false, nil)
		}
		err = handle(nv)
		assert.Error(t, err)

		err = handle(nv)
		assert.NoError(t, err)
	})
}

func TestMakeCertifiedMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	n := randN()
	r := randReplicaID(n)

	processViewMessage := func(msg messages.PeerMessage) (new bool, err error) {
		args := mock.MethodCalled("viewMessageProcessor", msg)
		return args.Bool(0), args.Error(1)
	}
	processViewChange := func(vc messages.ViewChange) (new bool, err error) {
		args := mock.MethodCalled("viewChangeProcessor", vc)
		return args.Bool(0), args.Error(1)
	}
	process := makeCertifiedMessageProcessor(n, processViewMessage, processViewChange)

	certifiedMsg := mock_messages.NewMockCertifiedMessage(ctrl)
	type peerMessage interface {
		ImplementsPeerMessage()
	}
	msg := struct {
		messages.CertifiedMessage
		peerMessage
	}{CertifiedMessage: certifiedMsg}

	// First certified message
	certifiedMsg.EXPECT().ReplicaID().Return(r)
	certifiedMsg.EXPECT().UI().Return(testUI(1))
	mock.On("viewMessageProcessor", msg).Return(true, nil).Once()
	new, err := process(msg)
	assert.NoError(t, err)
	assert.True(t, new)

	// First certified message from another replica
	otherReplica := randOtherReplicaID(r, n)
	certifiedMsg.EXPECT().ReplicaID().Return(otherReplica)
	certifiedMsg.EXPECT().UI().Return(testUI(1))
	mock.On("viewMessageProcessor", msg).Return(true, nil).Once()
	new, err = process(msg)
	assert.NoError(t, err)
	assert.True(t, new)

	certifiedMsg.EXPECT().ReplicaID().Return(r).AnyTimes()

	// Another certified message
	certifiedMsg.EXPECT().UI().Return(testUI(2))
	mock.On("viewMessageProcessor", msg).Return(true, nil).Once()
	new, err = process(msg)
	assert.NoError(t, err)
	assert.True(t, new)

	// Duplicate certified message
	certifiedMsg.EXPECT().UI().Return(testUI(2))
	new, err = process(msg)
	assert.NoError(t, err)
	assert.False(t, new)

	// Older certified message
	certifiedMsg.EXPECT().UI().Return(testUI(1))
	new, err = process(msg)
	assert.NoError(t, err)
	assert.False(t, new)

	// New certified, but non-sequential, message from replica r
	certifiedMsg.EXPECT().UI().Return(testUI(4))
	_, err = process(msg)
	assert.Error(t, err, "Non-sequential message")

	// Next certified, but redundant, message
	certifiedMsg.EXPECT().UI().Return(testUI(3))
	mock.On("viewMessageProcessor", msg).Return(false, nil).Once()
	new, err = process(msg)
	assert.NoError(t, err)
	assert.False(t, new)

	// Next certified, but incorrect, message
	certifiedMsg.EXPECT().UI().Return(testUI(4))
	mock.On("viewMessageProcessor", msg).Return(false, fmt.Errorf("error")).Once()
	_, err = process(msg)
	assert.Error(t, err, "Failed to process message in current view")

	// Next certified, but now unacceptable, message
	certifiedMsg.EXPECT().UI().Return(testUI(5))
	_, err = process(msg)
	assert.Error(t, err, "Certified message following an incorrect one")

	// ViewChange message from the other replica
	vc := MakeTestVC(messageImpl, otherReplica, 1, nil, nil, 2)
	mock.On("viewChangeProcessor", vc).Return(false, nil).Once()
	new, err = process(vc)
	assert.NoError(t, err)
	assert.False(t, new)

	// Another ViewChange message from the other replica
	vc = MakeTestVC(messageImpl, otherReplica, 2, nil, nil, 3)
	mock.On("viewChangeProcessor", vc).Return(true, nil).Once()
	new, err = process(vc)
	assert.NoError(t, err)
	assert.True(t, new)

	// Invalid ViewChange message from the other replica
	vc = MakeTestVC(messageImpl, otherReplica, 3, nil, nil, 4)
	mock.On("viewChangeProcessor", vc).Return(false, fmt.Errorf("error")).Once()
	_, err = process(vc)
	assert.Error(t, err)
}

func TestMakeViewMessageProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	viewState := mock_viewstate.NewMockState(ctrl)
	releaseView := func() {
		mock.MethodCalled("viewReleaser")
	}
	applyPeerMessage := func(msg messages.PeerMessage) error {
		args := mock.MethodCalled("peerMessageApplier", msg)
		return args.Error(0)
	}
	process := makeViewMessageProcessor(viewState, applyPeerMessage)

	n := randN()
	view := randView() + uint64(n)
	oldView := view - uint64(1+rand.Intn(int(n-1)))
	newView := view + uint64(1+rand.Intn(int(n-1)))
	primary := primaryID(n, view)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(primary, view, request)
	commit := messageImpl.NewCommit(randOtherReplicaID(primary, n), prepare)
	nv := messageImpl.NewNewView(primary, view, nil)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		assert.Panics(t, func() { process(msg) }, "Unknown message type")
	})

	testPeerMessage := func(t *testing.T, msg messages.PeerMessage) {
		viewState.EXPECT().HoldView().Return(oldView, oldView, releaseView)
		mock.On("viewReleaser").Once()
		_, err := process(msg)
		assert.Error(t, err, "Message for unexpected view")

		viewState.EXPECT().HoldView().Return(view, view, releaseView)
		mock.On("peerMessageApplier", msg).Return(nil).Once()
		mock.On("viewReleaser").Once()
		new, err := process(msg)
		assert.NoError(t, err)
		assert.True(t, new)

		viewState.EXPECT().HoldView().Return(view, newView, releaseView)
		mock.On("viewReleaser").Once()
		new, err = process(msg)
		assert.NoError(t, err)
		assert.False(t, new)

		viewState.EXPECT().HoldView().Return(newView, newView, releaseView)
		mock.On("viewReleaser").Once()
		new, err = process(msg)
		assert.NoError(t, err)
		assert.False(t, new)
	}
	t.Run("Prepare", func(t *testing.T) {
		testPeerMessage(t, prepare)
	})
	t.Run("Commit", func(t *testing.T) {
		testPeerMessage(t, commit)
	})
	t.Run("NewView", func(t *testing.T) {
		viewState.EXPECT().AdvanceCurrentView(view).Return(false, view, releaseView)
		new, err := process(nv)
		assert.NoError(t, err)
		assert.False(t, new)

		viewState.EXPECT().AdvanceCurrentView(view).Return(false, newView, releaseView)
		new, err = process(nv)
		assert.NoError(t, err)
		assert.False(t, new)

		viewState.EXPECT().AdvanceCurrentView(view).Return(true, view, releaseView)
		mock.On("viewReleaser").Once()
		mock.On("peerMessageApplier", nv).Return(nil).Once()
		new, err = process(nv)
		assert.NoError(t, err)
		assert.True(t, new)

		viewState.EXPECT().AdvanceCurrentView(view).Return(true, newView, releaseView)
		mock.On("viewReleaser").Once()
		new, err = process(nv)
		assert.NoError(t, err)
		assert.False(t, new)
	})
}

func TestMakePeerMessageApplier(t *testing.T) {
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
	applyNewView := func(msg messages.NewView) error {
		args := mock.MethodCalled("newViewApplier", msg)
		return args.Error(0)
	}
	apply := makePeerMessageApplier(applyPrepare, applyCommit, applyNewView)

	reqSeq := rand.Uint64()
	request := messageImpl.NewRequest(0, reqSeq, nil)
	prepare := messageImpl.NewPrepare(0, 0, request)
	commit := messageImpl.NewCommit(1, prepare)
	nv := messageImpl.NewNewView(1, 1, nil)

	t.Run("UnknownMessageType", func(t *testing.T) {
		msg := mock_messages.NewMockPeerMessage(ctrl)
		assert.Panics(t, func() { apply(msg) }, "Unknown message type")
	})
	t.Run("Prepare", func(t *testing.T) {
		mock.On("prepareApplier", prepare).Return(fmt.Errorf("error")).Once()
		err := apply(prepare)
		assert.Error(t, err, "Failed to apply Prepare")

		mock.On("prepareApplier", prepare).Return(nil).Once()
		err = apply(prepare)
		assert.NoError(t, err)
	})
	t.Run("Commit", func(t *testing.T) {
		mock.On("commitApplier", commit).Return(fmt.Errorf("error")).Once()
		err := apply(commit)
		assert.Error(t, err, "Failed to apply Commit")

		mock.On("commitApplier", commit).Return(nil).Once()
		err = apply(commit)
		assert.NoError(t, err)
	})
	t.Run("NewView", func(t *testing.T) {
		mock.On("newViewApplier", nv).Return(fmt.Errorf("error")).Once()
		err := apply(nv)
		assert.Error(t, err, "Failed to apply NewView")

		mock.On("newViewApplier", nv).Return(nil).Once()
		err = apply(nv)
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
	clientStates := mock_clientstate.NewMockProvider(ctrl)
	clientStates.EXPECT().ClientState(clientID).Return(clientState).AnyTimes()

	consume := makeGeneratedMessageConsumer(log, clientStates, logger.NewReplicaLogger(0))

	t.Run("Reply", func(t *testing.T) {
		reply := messageImpl.NewReply(rand.Uint32(), clientID, rand.Uint64(), nil)

		clientState.EXPECT().AddReply(reply)
		consume(reply)
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
