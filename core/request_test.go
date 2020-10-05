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
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
	"github.com/hyperledger-labs/minbft/core/internal/utils"
	"github.com/hyperledger-labs/minbft/messages"

	mock_api "github.com/hyperledger-labs/minbft/api/mocks"
	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
	mock_requestlist "github.com/hyperledger-labs/minbft/core/internal/requestlist/mocks"
	mock_viewstate "github.com/hyperledger-labs/minbft/core/internal/viewstate/mocks"
)

func TestMakeRequestProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	captureSeq := func(request messages.Request) (new bool, release func()) {
		args := mock.MethodCalled("requestSeqCapturer", request)
		return args.Bool(0), func() {
			mock.MethodCalled("requestSeqReleaser", request)
		}
	}
	applyRequest := func(request messages.Request, view uint64) error {
		args := mock.MethodCalled("requestApplier", request, view)
		return args.Error(0)
	}
	pendingReq := mock_requestlist.NewMockList(ctrl)
	viewState := mock_viewstate.NewMockState(ctrl)
	process := makeRequestProcessor(captureSeq, pendingReq, viewState, applyRequest)

	n := utils.RandN()
	view := utils.RandView()
	newView := view + uint64(1+rand.Intn(int(n-1)))
	request := messageImpl.NewRequest(0, rand.Uint64(), nil)

	mock.On("requestSeqCapturer", request).Return(false).Once()
	new, err := process(request)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("requestSeqCapturer", request).Return(true).Once()
	pendingReq.EXPECT().Add(request)
	viewState.EXPECT().HoldView().Return(view, newView, func() {
		mock.MethodCalled("viewReleaser")
	})
	mock.On("viewReleaser").Once()
	mock.On("requestSeqReleaser", request).Once()
	_, err = process(request)
	assert.NoError(t, err)

	mock.On("requestSeqCapturer", request).Return(true).Once()
	pendingReq.EXPECT().Add(request)
	viewState.EXPECT().HoldView().Return(view, view, func() {
		mock.MethodCalled("viewReleaser")
	})
	mock.On("requestApplier", request, view).Return(fmt.Errorf("Failed")).Once()
	mock.On("viewReleaser").Once()
	mock.On("requestSeqReleaser", request).Once()
	_, err = process(request)
	assert.Error(t, err, "Failed to apply Request")

	mock.On("requestSeqCapturer", request).Return(true).Once()
	pendingReq.EXPECT().Add(request)
	viewState.EXPECT().HoldView().Return(view, view, func() {
		mock.MethodCalled("viewReleaser")
	})
	mock.On("requestApplier", request, view).Return(nil).Once()
	mock.On("viewReleaser").Once()
	mock.On("requestSeqReleaser", request).Once()
	new, err = process(request)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeRequestApplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := utils.RandN()
	ownView := utils.RandView()
	otherView := utils.RandOtherView(ownView)
	id := utils.PrimaryID(n, ownView)

	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	startReqTimer := func(request messages.Request, view uint64) {
		mock.MethodCalled("requestTimerStarter", request, view)
	}
	startPrepTimer := func(request messages.Request, view uint64) {
		mock.MethodCalled("prepareTimerStarter", request, view)
	}
	apply := makeRequestApplier(id, n, handleGeneratedMessage, startReqTimer, startPrepTimer)

	clientID := rand.Uint32()
	request := messageImpl.NewRequest(clientID, rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(id, ownView, request)

	mock.On("requestTimerStarter", request, otherView).Once()
	mock.On("prepareTimerStarter", request, otherView).Once()
	err := apply(request, otherView)
	assert.NoError(t, err)

	mock.On("requestTimerStarter", request, ownView).Once()
	mock.On("generatedMessageHandler", prepare).Once()
	err = apply(request, ownView)
	assert.NoError(t, err)
}

func TestMakeRequestValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)

	verify := func(msg messages.SignedMessage) error {
		args := mock.MethodCalled("MessageSignatureVerifier", msg)
		return args.Error(0)
	}
	validate := makeRequestValidator(verify)

	mock.On("MessageSignatureVerifier", request).Return(fmt.Errorf("invalid signature")).Once()
	err := validate(request)
	assert.Error(t, err)

	mock.On("MessageSignatureVerifier", request).Return(nil).Once()
	err = validate(request)
	assert.NoError(t, err)
}

func TestMakeRequestExecutor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	seq := rand.Uint64()
	clientID := rand.Uint32()
	replicaID := rand.Uint32()

	expectedOperation := utils.RandBytes()
	expectedResult := utils.RandBytes()

	request := messageImpl.NewRequest(clientID, seq, expectedOperation)
	expectedReply := messageImpl.NewReply(replicaID, clientID, seq, expectedResult)

	retireSeq := func(request messages.Request) (new bool) {
		args := mock.MethodCalled("requestSeqRetirer", request)
		return args.Bool(0)
	}
	stopReqTimer := func(request messages.Request) {
		mock.MethodCalled("requestTimerStopper", request)
	}
	pendingReq := mock_requestlist.NewMockList(ctrl)
	consumer := mock_api.NewMockRequestConsumer(ctrl)
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	requestExecutor := makeRequestExecutor(replicaID, retireSeq, pendingReq, stopReqTimer, consumer, handleGeneratedMessage)

	mock.On("requestSeqRetirer", request).Return(false).Once()
	requestExecutor(request)

	resultChan := make(chan []byte, 1)
	resultChan <- expectedResult
	done := make(chan struct{})
	mock.On("requestSeqRetirer", request).Return(true).Once()
	pendingReq.EXPECT().Remove(clientID)
	mock.On("requestTimerStopper", request).Once()
	consumer.EXPECT().Deliver(expectedOperation).Return(resultChan)
	mock.On("generatedMessageHandler", expectedReply).Run(
		func(testifymock.Arguments) { close(done) },
	).Once()
	requestExecutor(request)
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
	request := messageImpl.NewRequest(expectedClientID, seq, nil)

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
	request := messageImpl.NewRequest(expectedClientID, seq, nil)

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
	request := messageImpl.NewRequest(expectedClientID, seq, nil)

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
	request := messageImpl.NewRequest(expectedClientID, seq, nil)
	reply := messageImpl.NewReply(rand.Uint32(), expectedClientID, seq, nil)

	replier := makeRequestReplier(provider)

	// Matching Reply sent after
	in := make(chan messages.Reply, 1)
	state.EXPECT().ReplyChannel(seq).Return(in)
	out := replier(request)
	in <- reply
	close(in)
	assert.Equal(t, reply, <-out)
	_, more := <-out
	assert.False(t, more, "Channel should be closed")

	// Matching Reply sent before
	in = make(chan messages.Reply, 1)
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
	seq := rand.Uint64()
	view := rand.Uint64()

	provider, state := setupClientStateProviderMock(t, ctrl, clientID)
	handleTimeout := func(view uint64) {
		mock.MethodCalled("requestTimeoutHandler", view)
	}

	startTimer := makeRequestTimerStarter(provider, handleTimeout,
		logging.MustGetLogger(module))

	request := messageImpl.NewRequest(clientID, seq, nil)

	state.EXPECT().StartRequestTimer(seq, gomock.Any()).Do(func(_ uint64, f func()) { f() })
	mock.On("requestTimeoutHandler", view).Once()
	startTimer(request, view)
}

func TestMakeRequestTimerStopper(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientID := rand.Uint32()
	seq := rand.Uint64()
	provider, state := setupClientStateProviderMock(t, ctrl, clientID)

	stopTimer := makeRequestTimerStopper(provider)

	request := messageImpl.NewRequest(clientID, seq, nil)

	state.EXPECT().StopRequestTimer(seq)
	stopTimer(request)
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

func TestMakePrepareTimerStarter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	clientID := rand.Uint32()
	seq := rand.Uint64()
	view := rand.Uint64()

	provider, state := setupClientStateProviderMock(t, ctrl, clientID)

	unicastLogs := map[uint32]messagelog.MessageLog{
		uint32(0): messagelog.New(),
	}
	startTimer := makePrepareTimerStarter(uint32(0), provider, unicastLogs, logging.MustGetLogger(module))

	request := messageImpl.NewRequest(clientID, seq, nil)

	state.EXPECT().StartPrepareTimer(seq, gomock.Any())
	startTimer(request, view)
}

func TestMakePrepareTimerStopper(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	clientID := rand.Uint32()
	seq := rand.Uint64()
	provider, state := setupClientStateProviderMock(t, ctrl, clientID)

	stopTimer := makePrepareTimerStopper(provider)

	request := messageImpl.NewRequest(clientID, seq, nil)

	state.EXPECT().StopPrepareTimer(seq)
	stopTimer(request)
}

func TestMakePrepareTimeoutProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedTimeout := time.Duration(rand.Int())
	config := mock_api.NewMockConfiger(ctrl)
	config.EXPECT().TimeoutPrepare().Return(expectedTimeout).AnyTimes()

	prepareTimeout := makePrepareTimeoutProvider(config)

	timeout := prepareTimeout()
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
