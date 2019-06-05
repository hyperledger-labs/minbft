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
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logging "github.com/op/go-logging"
	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/messages"

	mock_api "github.com/hyperledger-labs/minbft/api/mocks"
	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
)

func TestMakeRequestProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	captureSeq := func(request *messages.Request) (new bool, release func()) {
		args := mock.MethodCalled("requestSeqCapturer", request)
		return args.Bool(0), func() {
			mock.MethodCalled("requestSeqReleaser", request)
		}
	}
	applyRequest := func(request *messages.Request) error {
		args := mock.MethodCalled("requestApplier", request)
		return args.Error(0)
	}
	process := makeRequestProcessor(captureSeq, applyRequest)

	request := &messages.Request{
		Msg: &messages.Request_M{
			Seq: rand.Uint64(),
		},
	}

	mock.On("requestSeqCapturer", request).Return(false).Once()
	new, err := process(request)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("requestSeqCapturer", request).Return(true).Once()
	mock.On("requestApplier", request).Return(fmt.Errorf("Failed")).Once()
	mock.On("requestSeqReleaser", request).Once()
	_, err = process(request)
	assert.Error(t, err, "Failed to apply Request")

	mock.On("requestSeqCapturer", request).Return(true).Once()
	mock.On("requestApplier", request).Return(nil).Once()
	mock.On("requestSeqReleaser", request).Once()
	new, err = process(request)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeRequestApplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	ownView := randView()
	otherView := randOtherView(ownView)
	id := primaryID(n, ownView)

	provideView := func() uint64 {
		args := mock.MethodCalled("viewProvider")
		return args.Get(0).(uint64)
	}
	handleGeneratedUIMessage := func(msg messages.MessageWithUI) {
		mock.MethodCalled("generatedUIMessageHandler", msg)
	}
	startReqTimer := func(clientID uint32, view uint64) {
		mock.MethodCalled("requestTimerStarter", clientID, view)
	}
	apply := makeRequestApplier(id, n, provideView, handleGeneratedUIMessage, startReqTimer)

	clientID := rand.Uint32()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: clientID,
			Seq:      rand.Uint64(),
		},
	}
	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			View:      ownView,
			ReplicaId: id,
			Request:   request,
		},
	}

	mock.On("viewProvider").Return(otherView).Once()
	mock.On("requestTimerStarter", clientID, otherView).Once()
	err := apply(request)
	assert.NoError(t, err)

	mock.On("viewProvider").Return(ownView).Once()
	mock.On("requestTimerStarter", clientID, ownView).Once()
	mock.On("generatedUIMessageHandler", prepare).Once()
	err = apply(request)
	assert.NoError(t, err)
}

func TestMakeRequestValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: rand.Uint32(),
		},
	}

	verify := func(msg messages.MessageWithSignature) error {
		args := mock.MethodCalled("messageSignatureVerifier", msg)
		return args.Error(0)
	}
	validate := makeRequestValidator(verify)

	mock.On("messageSignatureVerifier", request).Return(fmt.Errorf("invalid signature")).Once()
	err := validate(request)
	assert.Error(t, err)

	mock.On("messageSignatureVerifier", request).Return(nil).Once()
	err = validate(request)
	assert.NoError(t, err)
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

	execute := func(operation []byte) <-chan []byte {
		args := mock.MethodCalled("operationExecutor", operation)
		return args.Get(0).(chan []byte)
	}
	sign := func(msg messages.MessageWithSignature) {
		mock.MethodCalled("replicaMessageSigner", msg)
		msg.AttachSignature(expectedSignature)
	}
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	requestExecutor := makeRequestExecutor(replicaID, execute, sign, handleGeneratedMessage)

	resultChan := make(chan []byte, 1)
	resultChan <- expectedResult
	done := make(chan struct{})
	mock.On("operationExecutor", expectedOperation).Return(resultChan).Once()
	mock.On("replicaMessageSigner", expectedUnsignedReply).Once()
	mock.On("generatedMessageHandler", expectedSignedReply).Run(
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

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

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

	state.EXPECT().CaptureRequestSeq(seq).Return(false, nil)
	new, _ := captureSeq(request)
	assert.False(t, new)

	state.EXPECT().CaptureRequestSeq(seq).Return(true, func() {
		mock.MethodCalled("requestSeqReleaser", seq)
	})
	new, release := captureSeq(request)
	assert.True(t, new)
	mock.On("requestSeqReleaser", seq).Once()
	release()
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

	state.EXPECT().PrepareRequestSeq(seq).Return(false, fmt.Errorf("Error"))
	assert.Panics(t, func() { prepareSeq(request) })

	state.EXPECT().PrepareRequestSeq(seq).Return(false, nil)
	new := prepareSeq(request)
	assert.False(t, new)

	state.EXPECT().PrepareRequestSeq(seq).Return(true, nil)
	new = prepareSeq(request)
	assert.True(t, new)
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

	state.EXPECT().RetireRequestSeq(seq).Return(false, fmt.Errorf("Error"))
	assert.Panics(t, func() { retireSeq(request) })

	state.EXPECT().RetireRequestSeq(seq).Return(false, nil)
	new := retireSeq(request)
	assert.False(t, new)

	state.EXPECT().RetireRequestSeq(seq).Return(true, nil)
	new = retireSeq(request)
	assert.True(t, new)
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

func TestMakeRequestTimerStarter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	clientID := rand.Uint32()
	view := rand.Uint64()
	provider, state := setupClientStateProviderMock(t, ctrl, clientID)
	handleTimeout := func(view uint64) {
		mock.MethodCalled("requestTimeoutHandler", view)
	}

	startTimer := makeRequestTimerStarter(provider, handleTimeout,
		logging.MustGetLogger(module))

	state.EXPECT().StartRequestTimer(gomock.Any()).Do(func(f func()) { f() })
	mock.On("requestTimeoutHandler", view).Once()
	startTimer(clientID, view)
}

func TestMakeRequestTimerStopper(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, clientID)

	stopTimer := makeRequestTimerStopper(provider)

	state.EXPECT().StopRequestTimer()
	stopTimer(clientID)
}

func TestMakeRequestTimeoutProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedTimeout := time.Duration(rand.Int())
	config := mock_api.NewMockConfiger(ctrl)
	config.EXPECT().TimeoutRequest().Return(expectedTimeout).AnyTimes()

	requestTimeout := makeRequestTimeoutProvider(config)

	timeout := requestTimeout()
	assert.Equal(t, expectedTimeout, timeout)
}

func setupClientStateProviderMock(t *testing.T, ctrl *gomock.Controller, expectedClientID uint32) (clientstate.Provider, *mock_clientstate.MockState) {
	state := mock_clientstate.NewMockState(ctrl)
	provider := func(clientID uint32) clientstate.State {
		require.Equal(t, expectedClientID, clientID)
		return state
	}

	return provider, state
}
