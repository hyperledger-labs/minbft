// Copyright (c) 2018-2019 NEC Laboratories Europe GmbH.
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

package clientstate

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/core/internal/timer"

	timermock "github.com/hyperledger-labs/minbft/core/internal/timer/mock"
)

func TestTimeout(t *testing.T) {
	t.Run("Start", testStartTimer)
	t.Run("Stop", testStopTimer)
	t.Run("StopAll", testStopAllTimers)
}

func testStartTimer(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, timerProvider, handleRequestTimeout, handlePrepareTimeout := setupTimeoutMock(mock, ctrl)

	seq := randSeq()

	// Start with disabled timer
	mock.On("requestTimeout").Return(time.Duration(0)).Once()
	s.StartRequestTimer(seq, handleRequestTimeout)

	mock.On("prepareTimeout").Return(time.Duration(0)).Once()
	s.StartPrepareTimer(seq, handlePrepareTimeout)

	// Start with enabled timer
	timeout := randTimeout()
	mockTimer := timermock.NewMockTimer(ctrl)
	timerProvider.EXPECT().AfterFunc(timeout, gomock.Any()).DoAndReturn(
		func(d time.Duration, f func()) timer.Timer {
			f()
			return mockTimer
		},
	).Times(2)
	mock.On("requestTimeout").Return(timeout).Once()
	mock.On("requestTimeoutHandler").Once()
	s.StartRequestTimer(seq, handleRequestTimeout)
	mock.On("prepareTimeout").Return(timeout).Once()
	mock.On("prepareTimeoutHandler").Once()
	s.StartPrepareTimer(seq, handlePrepareTimeout)

	// Restart timer
	mockTimer.EXPECT().Stop().Return(true).Times(2)
	timeout = randTimeout()
	mockTimer = timermock.NewMockTimer(ctrl)
	timerProvider.EXPECT().AfterFunc(timeout, gomock.Any()).DoAndReturn(
		func(d time.Duration, f func()) timer.Timer {
			f()
			return mockTimer
		},
	).Times(2)
	mock.On("requestTimeout").Return(timeout).Once()
	mock.On("requestTimeoutHandler").Once()
	s.StartRequestTimer(seq, handleRequestTimeout)
	mock.On("prepareTimeout").Return(timeout).Once()
	mock.On("prepareTimeoutHandler").Once()
	s.StartPrepareTimer(seq, handlePrepareTimeout)
}

func testStopTimer(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, timerProvider, handleRequestTimeout, handlePrepareTimeout := setupTimeoutMock(mock, ctrl)

	timeout := randTimeout()
	mock.On("requestTimeout").Return(timeout)
	mock.On("prepareTimeout").Return(timeout)

	// Stop before started
	assert.NotPanics(t, func() {
		s.StopRequestTimer(0)
		s.StopPrepareTimer(0)
	})

	// Start timers
	mockTimer := timermock.NewMockTimer(ctrl)
	timerProvider.EXPECT().AfterFunc(timeout, gomock.Any()).Return(mockTimer).Times(2)
	seq := randSeq()
	s.StartRequestTimer(seq, handleRequestTimeout)
	s.StartPrepareTimer(seq, handlePrepareTimeout)

	// Stop with other request identifier
	seq2 := randOtherSeq(seq)
	s.StopRequestTimer(seq2)
	s.StopPrepareTimer(seq2)

	// Stop timers
	mockTimer.EXPECT().Stop().Return(true).Times(2)
	s.StopRequestTimer(seq)
	s.StopPrepareTimer(seq)

	// Stop again
	s.StopRequestTimer(seq)
	s.StopPrepareTimer(seq)
}

func testStopAllTimers(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, timerProvider, handleRequestTimeout, handlePrepareTimeout := setupTimeoutMock(mock, ctrl)

	timeout := randTimeout()
	mock.On("requestTimeout").Return(timeout)
	mock.On("prepareTimeout").Return(timeout)

	assert.NotPanics(t, func() { s.StopAllTimers() })

	// Start timers
	mockTimer := timermock.NewMockTimer(ctrl)
	timerProvider.EXPECT().AfterFunc(timeout, gomock.Any()).Return(mockTimer).Times(2)
	seq := randSeq()
	s.StartRequestTimer(seq, handleRequestTimeout)
	s.StartPrepareTimer(seq, handlePrepareTimeout)

	// Stop all timers
	mockTimer.EXPECT().Stop().Return(true).Times(2)
	s.StopAllTimers()
}

func setupTimeoutMock(mock *testifymock.Mock, ctrl *gomock.Controller) (state State, timerProvider *timermock.MockProvider, handleRequestTimeout func(), handlePrepareTimeout func()) {
	handleRequestTimeout = func() {
		mock.MethodCalled("requestTimeoutHandler")
	}
	handlePrepareTimeout = func() {
		mock.MethodCalled("prepareTimeoutHandler")
	}
	requestTimeout := func() time.Duration {
		args := mock.MethodCalled("requestTimeout")
		return args.Get(0).(time.Duration)
	}
	prepareTimeout := func() time.Duration {
		args := mock.MethodCalled("prepareTimeout")
		return args.Get(0).(time.Duration)
	}
	timerProvider = timermock.NewMockProvider(ctrl)
	state = newClientState(timerProvider, requestTimeout, prepareTimeout)
	return state, timerProvider, handleRequestTimeout, handlePrepareTimeout
}

func randTimeout() time.Duration {
	return time.Duration(rand.Intn(math.MaxInt32-1) + 1) // positive nonzero
}

func randSeq() uint64 {
	return rand.Uint64()
}

func randOtherSeq(seq uint64) uint64 {
	for {
		otherSeq := rand.Uint64()
		if otherSeq != seq {
			return otherSeq
		}
	}
}
