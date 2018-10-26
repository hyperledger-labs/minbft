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
	handle := setupMakeRequestHandlerMock(mock, id, n, view)
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

	// Invalid client signature
	mock.On("messageSignatureVerifier", request).Return(fmt.Errorf("invalid signature")).Once()
	_, err := handle(request)
	assert.Error(t, err)

	// Already captured request ID
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqCapturer", request).Return(false).Once()
	new, err := handle(request)
	assert.NoError(t, err)
	assert.False(t, new)

	// New Request
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqCapturer", request).Return(true).Once()
	mock.On("generatedUIMessageHandler", prepare).Once()
	mock.On("requestSeqReleaser", request).Once()
	mock.On("requestSeqPreparer", request).Return(nil).Once()
	new, err = handle(request)
	assert.NoError(t, err)
	assert.True(t, new)
}

func testMakeRequestHandlerBackup(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	id := randBackupID(n, view)
	handle := setupMakeRequestHandlerMock(mock, id, n, view)
	clientID := rand.Uint32()
	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: clientID,
			Seq:      seq,
		},
	}

	// Invalid client signature
	mock.On("messageSignatureVerifier", request).Return(fmt.Errorf("invalid signature")).Once()
	_, err := handle(request)
	assert.Error(t, err)

	// Already captured request ID
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqCapturer", request).Return(false).Once()
	new, err := handle(request)
	assert.NoError(t, err)
	assert.False(t, new)

	// New Request
	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	mock.On("requestSeqCapturer", request).Return(true).Once()
	mock.On("requestSeqReleaser", request).Once()
	new, err = handle(request)
	assert.NoError(t, err)
	assert.True(t, new)
}

func setupMakeRequestHandlerMock(mock *testifymock.Mock, id, n uint32, view uint64) requestHandler {
	provideView := func() uint64 {
		args := mock.MethodCalled("viewProvider")
		return args.Get(0).(uint64)
	}
	verify := func(msg messages.MessageWithSignature) error {
		args := mock.MethodCalled("messageSignatureVerifier", msg)
		return args.Error(0)
	}
	captureSeq := func(request *messages.Request) (new bool) {
		args := mock.MethodCalled("requestSeqCapturer", request)
		return args.Bool(0)
	}
	releaseSeq := func(request *messages.Request) {
		mock.MethodCalled("requestSeqReleaser", request)
	}
	prepareSeq := func(request *messages.Request) error {
		args := mock.MethodCalled("requestSeqPreparer", request)
		return args.Error(0)
	}
	handleGeneratedUIMessage := func(msg messages.MessageWithUI) {
		mock.MethodCalled("generatedUIMessageHandler", msg)
	}
	mock.On("viewProvider").Return(view)
	return makeRequestHandler(id, n, provideView, verify, captureSeq, releaseSeq, prepareSeq, handleGeneratedUIMessage)
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
			ClientId:  clientID,
			Seq:       seq,
			Result:    expectedResult,
		},
	}
	expectedSignedReply := &messages.Reply{
		Msg:       expectedUnsignedReply.Msg,
		Signature: expectedSignature,
	}

	mockOperationExecutor := func(operation []byte) <-chan []byte {
		args := mock.MethodCalled("operationExecutor", operation)
		return args.Get(0).(chan []byte)
	}
	mockReplicaMessageSigner := func(msg messages.MessageWithSignature) {
		mock.MethodCalled("replicaMessageSigner", msg)
		msg.AttachSignature(expectedSignature)
	}
	mockReplyConsumer := func(reply *messages.Reply) {
		mock.MethodCalled("replyConsumer", reply)
	}

	requestExecutor := makeRequestExecutor(replicaID,
		mockOperationExecutor, mockReplicaMessageSigner, mockReplyConsumer)

	resultChan := make(chan []byte, 1)
	resultChan <- expectedResult
	done := make(chan struct{})
	mock.On("operationExecutor", expectedOperation).Return(resultChan).Once()
	mock.On("replicaMessageSigner", expectedUnsignedReply).Once()
	mock.On("replyConsumer", expectedSignedReply).Run(
		func(testifymock.Arguments) { close(done) },
	).Once()
	requestExecutor(request)
	<-done
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
	resChan := make(chan []byte, 1)

	// Normal execution
	resChan <- expectedRes
	consumer.EXPECT().Deliver(op).Return(resChan)
	res := <-executor(op)
	assert.Equal(t, expectedRes, res)

	// Concurrent execution
	started := make(chan struct{})
	exit := make(chan struct{})
	done := make(chan struct{})
	consumer.EXPECT().Deliver(op).Return(resChan).Do(func([]byte) {
		close(started)
		<-exit
	})
	go func() {
		res := <-executor(op)
		assert.Equal(t, expectedRes, res)
		done <- struct{}{}
	}()
	<-started
	assert.Panics(t, func() {
		_ = executor(op)
	})
	close(exit)
	resChan <- expectedRes
	<-done
}

func TestMakeRequestSeqCapturer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, expectedClientID)

	captureSeq := makeRequestSeqCapturer(provider)

	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: expectedClientID,
			Seq:      seq,
		},
	}

	state.EXPECT().CaptureRequestSeq(seq).Return(false)
	new := captureSeq(request)
	assert.False(t, new)

	state.EXPECT().CaptureRequestSeq(seq).Return(true)
	new = captureSeq(request)
	assert.True(t, new)
}

func TestMakeRequestSeqReleaser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, expectedClientID)

	releaseSeq := makeRequestSeqReleaser(provider)

	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: expectedClientID,
			Seq:      seq,
		},
	}

	state.EXPECT().ReleaseRequestSeq(seq).Return(fmt.Errorf("Invalid ID"))
	assert.Panics(t, func() { releaseSeq(request) })

	state.EXPECT().ReleaseRequestSeq(seq).Return(nil)
	assert.NotPanics(t, func() { releaseSeq(request) })
}

func TestMakeRequestSeqPreparer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, expectedClientID)

	prepareSeq := makeRequestSeqPreparer(provider)

	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: expectedClientID,
			Seq:      seq,
		},
	}

	state.EXPECT().PrepareRequestSeq(seq).Return(fmt.Errorf("old request ID"))
	err := prepareSeq(request)
	assert.Error(t, err)

	state.EXPECT().PrepareRequestSeq(seq).Return(nil)
	err = prepareSeq(request)
	assert.NoError(t, err)
}

func TestMakeRequestSeqRetirer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, expectedClientID)

	retireSeq := makeRequestSeqRetirer(provider)

	seq := rand.Uint64()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: expectedClientID,
			Seq:      seq,
		},
	}

	state.EXPECT().RetireRequestSeq(seq).Return(fmt.Errorf("old request ID"))
	err := retireSeq(request)
	assert.Error(t, err)

	state.EXPECT().RetireRequestSeq(seq).Return(nil)
	err = retireSeq(request)
	assert.NoError(t, err)
}

func TestMakeRequestReplier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, expectedClientID)

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

	clientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, clientID)

	consumer := makeReplyConsumer(provider)

	seq := rand.Uint64()
	reply := &messages.Reply{
		Msg: &messages.Reply_M{
			ReplicaId: rand.Uint32(),
			ClientId:  clientID,
			Seq:       seq,
		},
	}

	state.EXPECT().AddReply(reply).Return(nil)
	assert.NotPanics(t, func() { consumer(reply) })

	state.EXPECT().AddReply(reply).Return(fmt.Errorf("Invalid request ID"))
	assert.Panics(t, func() { consumer(reply) })
}

func setupClientStateProviderMock(t *testing.T, ctrl *gomock.Controller, expectedClientID uint32) (clientstate.Provider, *mock_clientstate.MockState) {
	state := mock_clientstate.NewMockState(ctrl)
	provider := func(clientID uint32) clientstate.State {
		require.Equal(t, expectedClientID, clientID)
		return state
	}

	return provider, state
}
