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
	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/messages"

	mock_api "github.com/hyperledger-labs/minbft/api/mocks"
	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
)

func TestMakeRequestHandler(t *testing.T) {
	t.Run("Primary", testMakeRequestHandlerPrimary)
	t.Run("Backup", testMakeRequestHandlerBackup)
}

func testMakeRequestHandlerPrimary(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	id := primaryID(n, view)
	handler := setupMakeRequestHandlerMock(mock, id, n, view)
	clientID := rand.Uint32()
	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: clientID,
			Seq:      seq,
		},
	}
	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			View:      view,
			ReplicaId: uint32(id),
			Request:   request,
		},
	}

	// Invalid client signature from client
	err := fmt.Errorf("invalid signature")
	mock.On("messageSignatureVerifier", request).Return(err).Once()
	new, err := handler(request, false)
	assert.False(t, new)
	assert.Error(t, err)

	// Invalid client signature from Prepare
	err = fmt.Errorf("invalid signature")
	mock.On("messageSignatureVerifier", request).Return(err).Once()
	new, err = handler(request, true)
	assert.False(t, new)
	assert.Error(t, err)

	// Wrong request ID from client
	err = fmt.Errorf("invalid request ID")
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, true).Return(false, err).Once()
	new, err = handler(request, false)
	assert.False(t, new)
	assert.Error(t, err)

	// Wrong request ID from Prepare
	err = fmt.Errorf("invalid request ID")
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, true).Return(false, err).Once()
	new, err = handler(request, true)
	assert.False(t, new)
	assert.Error(t, err)

	// Already accepted request ID from Prepare
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, true).Return(false, nil).Once()
	new, err = handler(request, true)
	assert.False(t, new)
	assert.NoError(t, err)

	// New Request from client
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, true).Return(true, nil).Once()
	mock.On("generatedUIMessageHandler", prepare).Once()
	new, err = handler(request, false)
	assert.True(t, new)
	assert.NoError(t, err)
}

func testMakeRequestHandlerBackup(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	id := randBackupID(n, view)
	handler := setupMakeRequestHandlerMock(mock, id, n, view)
	clientID := rand.Uint32()
	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: clientID,
			Seq:      seq,
		},
	}

	// Invalid client signature from client
	err := fmt.Errorf("invalid signature")
	mock.On("messageSignatureVerifier", request).Return(err).Once()
	new, err := handler(request, false)
	assert.False(t, new)
	assert.Error(t, err)

	// Invalid client signature from Prepare
	err = fmt.Errorf("invalid signature")
	mock.On("messageSignatureVerifier", request).Return(err).Once()
	new, err = handler(request, true)
	assert.False(t, new)
	assert.Error(t, err)

	// Wrong request ID from client
	err = fmt.Errorf("invalid request ID")
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, false).Return(false, err).Once()
	new, err = handler(request, false)
	assert.False(t, new)
	assert.Error(t, err)

	// Wrong request ID from Prepare
	err = fmt.Errorf("invalid request ID")
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, true).Return(false, err).Once()
	new, err = handler(request, true)
	assert.False(t, new)
	assert.Error(t, err)

	// Already accepted request ID from client
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, false).Return(false, nil).Once()
	new, err = handler(request, false)
	assert.False(t, new)
	assert.NoError(t, err)

	// New Request from client
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, false).Return(true, nil).Once()
	new, err = handler(request, false)
	assert.True(t, new)
	assert.NoError(t, err)

	// New Request from Prepare
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqAcceptor", request, true).Return(true, nil).Once()
	new, err = handler(request, true)
	assert.True(t, new)
	assert.NoError(t, err)
}

func setupMakeRequestHandlerMock(mock *testifymock.Mock, id, n uint32, view uint64) requestHandler {
	provideView := func() uint64 {
		args := mock.MethodCalled("viewProvider")
		return args.Get(0).(uint64)
	}
	verifier := func(msg messages.MessageWithSignature) error {
		args := mock.MethodCalled("messageSignatureVerifier", msg)
		return args.Error(0)
	}
	seqAcceptor := func(request *messages.Request, prepared bool) (accepted bool, err error) {
		args := mock.MethodCalled("requestSeqAcceptor", request, prepared)
		return args.Bool(0), args.Error(1)
	}
	handleGeneratedUIMessage := func(msg messages.MessageWithUI) {
		mock.MethodCalled("generatedUIMessageHandler", msg)
	}
	mock.On("viewProvider").Return(view)
	return makeRequestHandler(id, n, provideView, verifier, seqAcceptor, handleGeneratedUIMessage)
}

func TestMakeRequestExecutor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	seq := rand.Uint64()
	clientID := rand.Uint32()
	replicaID := rand.Uint32()

	expectedOperation := make([]byte, 1)
	expectedResult := make([]byte, 1)
	expectedSignature := make([]byte, 1)
	rand.Read(expectedOperation)
	rand.Read(expectedResult)
	rand.Read(expectedSignature)

	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: clientID,
			Seq:      seq,
			Payload:  expectedOperation,
		},
	}
	expectedUnsignedReply := &messages.Reply{
		Msg: &messages.Reply_M{
			ReplicaId: replicaID,
			Seq:       seq,
			Result:    expectedResult,
		},
	}
	expectedSignedReply := &messages.Reply{
		Msg:       expectedUnsignedReply.Msg,
		Signature: expectedSignature,
	}

	mockOperationExecutor := func(operation []byte) []byte {
		args := mock.MethodCalled("operationExecutor", operation)
		return args.Get(0).([]byte)
	}
	mockReplicaMessageSigner := func(msg messages.MessageWithSignature) {
		mock.MethodCalled("replicaMessageSigner", msg)
		msg.AttachSignature(expectedSignature)
	}
	mockReplyConsumer := func(reply *messages.Reply, clientID uint32) {
		mock.MethodCalled("replyConsumer", reply, clientID)
	}

	requestExecutor := makeRequestExecutor(replicaID,
		mockOperationExecutor, mockReplicaMessageSigner, mockReplyConsumer)

	mock.On("operationExecutor", expectedOperation).Return(expectedResult).Once()
	mock.On("replicaMessageSigner", expectedUnsignedReply).Once()
	mock.On("replyConsumer", expectedSignedReply, clientID).Once()
	requestExecutor(request)
}

func TestMakeOperationExecutor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	consumer := mock_api.NewMockRequestConsumer(ctrl)
	executor := makeOperationExecutor(consumer)

	op := make([]byte, 1)
	expectedRes := make([]byte, 1)
	rand.Read(op)
	rand.Read(expectedRes)

	started := make(chan struct{})
	executed := make(chan struct{})
	done := make(chan struct{})
	consumer.EXPECT().Deliver(op).Return(expectedRes).Do(func([]byte) {
		started <- struct{}{}
		<-executed
	}).AnyTimes()
	runExecutor := func() {
		res := executor(op)
		assert.Equal(t, expectedRes, res)
		done <- struct{}{}
	}

	// Normal execution
	go runExecutor()
	<-started
	executed <- struct{}{}
	<-done

	// Concurrent execution
	go runExecutor()
	<-started
	assert.Panics(t, func() {
		_ = executor(op)
	})
	executed <- struct{}{}
	<-done
}

func TestMakeRequestSeqAcceptor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	state := mock_clientstate.NewMockState(ctrl)
	provider := func(clientID uint32) clientstate.State {
		require.Equal(t, expectedClientID, clientID)
		return state
	}

	acceptor := makeRequestSeqAcceptor(provider)

	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: expectedClientID,
			Seq:      seq,
		},
	}

	state.EXPECT().CheckRequestSeq(seq).Return(false, nil)
	new, err := acceptor(request, false)
	assert.NoError(t, err)
	assert.False(t, new)

	state.EXPECT().CheckRequestSeq(seq).Return(true, nil)
	new, err = acceptor(request, false)
	assert.NoError(t, err)
	assert.True(t, new)

	state.EXPECT().AcceptRequestSeq(seq).Return(false, nil)
	new, err = acceptor(request, true)
	assert.NoError(t, err)
	assert.False(t, new)

	state.EXPECT().AcceptRequestSeq(seq).Return(true, nil)
	new, err = acceptor(request, true)
	assert.NoError(t, err)
	assert.True(t, new)

	expectedErr := fmt.Errorf("Invalid request ID")

	state.EXPECT().CheckRequestSeq(seq).Return(false, expectedErr)
	new, err = acceptor(request, false)
	assert.Equal(t, expectedErr, err)
	assert.False(t, new)

	state.EXPECT().AcceptRequestSeq(seq).Return(false, expectedErr)
	new, err = acceptor(request, true)
	assert.Equal(t, expectedErr, err)
	assert.False(t, new)
}

func TestMakeRequestReplier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	state := mock_clientstate.NewMockState(ctrl)
	provider := func(clientID uint32) clientstate.State {
		require.Equal(t, expectedClientID, clientID)
		return state
	}

	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: expectedClientID,
			Seq:      seq,
		},
	}
	reply := &messages.Reply{
		Msg: &messages.Reply_M{
			ReplicaId: rand.Uint32(),
			Seq:       seq,
		},
	}

	replier := makeRequestReplier(provider)

	// Matching Reply sent after
	in := make(chan *messages.Reply, 1)
	state.EXPECT().ReplyChannel(seq).Return(in)
	out := replier(request)
	in <- reply
	close(in)
	assert.Equal(t, reply, <-out)
	_, more := <-out
	assert.False(t, more, "Channel should be closed")

	// Matching Reply sent before
	in = make(chan *messages.Reply, 1)
	in <- reply
	close(in)
	state.EXPECT().ReplyChannel(seq).Return(in)
	out = replier(request)
	assert.Equal(t, reply, <-out)
	_, more = <-out
	assert.False(t, more, "Channel should be closed")
}

func TestMakeReplyConsumer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	state := mock_clientstate.NewMockState(ctrl)
	provider := func(clientID uint32) clientstate.State {
		require.Equal(t, expectedClientID, clientID)
		return state
	}

	consumer := makeReplyConsumer(provider)

	seq := rand.Uint64()
	reply := &messages.Reply{
		Msg: &messages.Reply_M{
			ReplicaId: rand.Uint32(),
			Seq:       seq,
		},
	}

	state.EXPECT().AddReply(reply).Return(nil)
	assert.NotPanics(t, func() { consumer(reply, expectedClientID) })

	state.EXPECT().AddReply(reply).Return(fmt.Errorf("Invalid request ID"))
	assert.Panics(t, func() { consumer(reply, expectedClientID) })
}
