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

	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/common/logger"
	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
	"github.com/hyperledger-labs/minbft/messages"

	mock_api "github.com/hyperledger-labs/minbft/api/mocks"
	mock_clientstate "github.com/hyperledger-labs/minbft/core/internal/clientstate/mocks"
	mock_requestlist "github.com/hyperledger-labs/minbft/core/internal/requestlist/mocks"
	mock_viewstate "github.com/hyperledger-labs/minbft/core/internal/viewstate/mocks"

	. "github.com/hyperledger-labs/minbft/messages/testing"
)

func TestMakeRequestProcessor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	captureReq := func(request messages.Request) (new bool, release func()) {
		args := mock.MethodCalled("requestCapturer", request)
		return args.Bool(0), func() {
			mock.MethodCalled("requestReleaser", request)
		}
	}
	applyRequest := func(request messages.Request, view uint64) error {
		args := mock.MethodCalled("requestApplier", request, view)
		return args.Error(0)
	}
	viewState := mock_viewstate.NewMockState(ctrl)
	process := makeRequestProcessor(captureReq, viewState, applyRequest)

	n := randN()
	view := randView()
	newView := view + uint64(1+rand.Intn(int(n-1)))
	request := messageImpl.NewRequest(0, rand.Uint64(), nil)

	viewState.EXPECT().HoldView().Return(view, newView, func() {
		mock.MethodCalled("viewReleaser")
	})
	mock.On("requestCapturer", request).Return(false).Once()
	mock.On("viewReleaser").Once()
	new, err := process(request)
	assert.NoError(t, err)
	assert.False(t, new)

	viewState.EXPECT().HoldView().Return(view, newView, func() {
		mock.MethodCalled("viewReleaser")
	})
	mock.On("requestCapturer", request).Return(true).Once()
	mock.On("requestReleaser", request).Once()
	mock.On("viewReleaser").Once()
	_, err = process(request)
	assert.NoError(t, err)

	viewState.EXPECT().HoldView().Return(view, view, func() {
		mock.MethodCalled("viewReleaser")
	})
	mock.On("requestCapturer", request).Return(true).Once()
	mock.On("requestApplier", request, view).Return(fmt.Errorf("error")).Once()
	mock.On("requestReleaser", request).Once()
	mock.On("viewReleaser").Once()
	_, err = process(request)
	assert.Error(t, err, "Failed to apply Request")

	viewState.EXPECT().HoldView().Return(view, view, func() {
		mock.MethodCalled("viewReleaser")
	})
	mock.On("requestCapturer", request).Return(true).Once()
	mock.On("requestApplier", request, view).Return(nil).Once()
	mock.On("requestReleaser", request).Once()
	mock.On("viewReleaser").Once()
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

	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	startReqTimer := func(request messages.Request, view uint64) {
		mock.MethodCalled("requestTimerStarter", request, view)
	}
	startPrepTimer := func(request messages.Request, view uint64) {
		mock.MethodCalled("prepareTimerStarter", request, view)
	}
	apply := makeRequestApplier(id, n, startReqTimer, startPrepTimer, handleGeneratedMessage)

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

func TestMakePendingRequestApplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pendingReqList := mock_requestlist.NewMockList(ctrl)
	applyRequest := func(req messages.Request, view uint64) error {
		args := mock.MethodCalled("requestApplier", req, view)
		return args.Error(0)
	}
	apply := makePendingRequestApplier(pendingReqList, applyRequest)

	view := randView()
	reqs := []messages.Request{RandReq(messageImpl), RandReq(messageImpl)}
	pendingReqList.EXPECT().All().Return(reqs).AnyTimes()

	for _, req := range reqs {
		mock.On("requestApplier", req, view).Return(nil).Once()
	}
	apply(view)
}

func TestMakeRequestValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)

	verify := func(msg messages.SignedMessage) error {
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	seq := rand.Uint64()
	clientID := rand.Uint32()
	replicaID := rand.Uint32()

	expectedOperation := randBytes()
	expectedResult := randBytes()

	request := messageImpl.NewRequest(clientID, seq, expectedOperation)
	expectedReply := messageImpl.NewReply(replicaID, clientID, seq, expectedResult)

	retireReq := func(request messages.Request) (new bool) {
		args := mock.MethodCalled("requestRetirer", request)
		return args.Bool(0)
	}
	stopReqTimer := func(request messages.Request) {
		mock.MethodCalled("requestTimerStopper", request)
	}
	consumer := mock_api.NewMockRequestConsumer(ctrl)
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	requestExecutor := makeRequestExecutor(replicaID, retireReq, stopReqTimer, consumer, handleGeneratedMessage)

	mock.On("requestRetirer", request).Return(false).Once()
	requestExecutor(request)

	resultChan := make(chan []byte, 1)
	resultChan <- expectedResult
	done := make(chan struct{})
	mock.On("requestRetirer", request).Return(true).Once()
	mock.On("requestTimerStopper", request).Once()
	consumer.EXPECT().Deliver(expectedOperation).Return(resultChan)
	mock.On("generatedMessageHandler", expectedReply).Run(
		func(testifymock.Arguments) { close(done) },
	).Once()
	requestExecutor(request)
	<-done
}

func TestMakeRequestCapturer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	expectedClientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, expectedClientID)
	pendingReqs := mock_requestlist.NewMockList(ctrl)
	captureReq := makeRequestCapturer(provider, pendingReqs)

	seq := rand.Uint64()
	request := messageImpl.NewRequest(expectedClientID, seq, nil)

	state.EXPECT().CaptureRequestSeq(seq).Return(false, nil)
	new, _ := captureReq(request)
	assert.False(t, new)

	state.EXPECT().CaptureRequestSeq(seq).Return(true, func() {
		mock.MethodCalled("requestSeqReleaser", seq)
	})
	pendingReqs.EXPECT().Add(request)
	new, release := captureReq(request)
	assert.True(t, new)
	mock.On("requestSeqReleaser", seq).Once()
	release()
}

func TestMakeRequestPreparer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	expectedClientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, expectedClientID)

	prepareReq := makeRequestPreparer(provider)

	seq := rand.Uint64()
	request := messageImpl.NewRequest(expectedClientID, seq, nil)

	state.EXPECT().PrepareRequestSeq(seq).Return(false, fmt.Errorf("error"))
	assert.Panics(t, func() { prepareReq(request) })

	state.EXPECT().PrepareRequestSeq(seq).Return(false, nil)
	new := prepareReq(request)
	assert.False(t, new)

	state.EXPECT().PrepareRequestSeq(seq).Return(true, nil)
	new = prepareReq(request)
	assert.True(t, new)
}

func TestMakeRequestRetirer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	expectedClientID := rand.Uint32()
	provider, state := setupClientStateProviderMock(t, ctrl, expectedClientID)
	pendingReqs := mock_requestlist.NewMockList(ctrl)
	retireReq := makeRequestRetirer(provider, pendingReqs)

	seq := rand.Uint64()
	request := messageImpl.NewRequest(expectedClientID, seq, nil)

	state.EXPECT().RetireRequestSeq(seq).Return(false, fmt.Errorf("error"))
	assert.Panics(t, func() { retireReq(request) })

	state.EXPECT().RetireRequestSeq(seq).Return(false, nil)
	new := retireReq(request)
	assert.False(t, new)

	state.EXPECT().RetireRequestSeq(seq).Return(true, nil)
	pendingReqs.EXPECT().Remove(request)
	new = retireReq(request)
	assert.True(t, new)
}

func TestMakeRequestUnpreparer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ids := []uint32{rand.Uint32(), rand.Uint32()}
	provider := mock_clientstate.NewMockProvider(ctrl)
	provider.EXPECT().Clients().Return(ids).AnyTimes()
	states := make([]*mock_clientstate.MockState, len(ids))
	for i, c := range ids {
		states[i] = mock_clientstate.NewMockState(ctrl)
		provider.EXPECT().ClientState(c).Return(states[i]).AnyTimes()
	}

	unprepareReqs := makeRequestUnpreparer(provider)

	for _, s := range states {
		s.EXPECT().UnprepareRequestSeq()
	}
	unprepareReqs()
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

	startTimer := makeRequestTimerStarter(provider, handleTimeout, logger.NewReplicaLogger(clientID))

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
	startTimer := makePrepareTimerStarter(uint32(0), provider, unicastLogs, logger.NewReplicaLogger(0))

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
	provider := mock_clientstate.NewMockProvider(ctrl)
	provider.EXPECT().ClientState(expectedClientID).Return(state).AnyTimes()

	return provider, state
}
