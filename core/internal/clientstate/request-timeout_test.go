// Copyright (c) 2018 NEC Laboratories Europe GmbH.
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

func TestRequestTimeout(t *testing.T) {
	t.Run("Start", testStartRequestTimeout)
	t.Run("Stop", testStopRequestTimeout)
}

func testStartRequestTimeout(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, timerProvider, handleTimeout := setupRequestTimeoutMock(mock, ctrl)

	// Start with disabled request timeout
	mock.On("requestTimeout").Return(time.Duration(0)).Once()
	s.StartRequestTimer(handleTimeout)

	// Start with enabled request timeout
	timeout := randTimeout()
	mock.On("requestTimeout").Return(timeout).Once()
	mockTimer := timermock.NewMockTimer(ctrl)
	timerProvider.EXPECT().AfterFunc(timeout, gomock.Any()).DoAndReturn(
		func(d time.Duration, f func()) timer.Timer {
			f()
			return mockTimer
		},
	)
	mock.On("requestTimeoutHandler").Once()
	s.StartRequestTimer(handleTimeout)

	// Restart request timeout
	mockTimer.EXPECT().Stop()
	timeout = randTimeout()
	mock.On("requestTimeout").Return(timeout).Once()
	mockTimer = timermock.NewMockTimer(ctrl)
	timerProvider.EXPECT().AfterFunc(timeout, gomock.Any()).DoAndReturn(
		func(d time.Duration, f func()) timer.Timer {
			f()
			return mockTimer
		},
	)
	mock.On("requestTimeoutHandler").Once()
	s.StartRequestTimer(handleTimeout)
}

func testStopRequestTimeout(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, timerProvider, handleTimeout := setupRequestTimeoutMock(mock, ctrl)

	timeout := randTimeout()
	mock.On("requestTimeout").Return(timeout)

	// Stop before started
	assert.NotPanics(t, func() {
		s.StopRequestTimer()
	})

	// Start and stop
	mockTimer := timermock.NewMockTimer(ctrl)
	timerProvider.EXPECT().AfterFunc(timeout, gomock.Any()).Return(mockTimer)
	s.StartRequestTimer(handleTimeout)
	mockTimer.EXPECT().Stop()
	s.StopRequestTimer()

	// Stop again
	mockTimer.EXPECT().Stop().AnyTimes()
	s.StopRequestTimer()
}

func setupRequestTimeoutMock(mock *testifymock.Mock, ctrl *gomock.Controller) (state State, timerProvider *timermock.MockProvider, handleTimeout func()) {
	handleTimeout = func() {
		mock.MethodCalled("requestTimeoutHandler")
	}
	requestTimeout := func() time.Duration {
		args := mock.MethodCalled("requestTimeout")
		return args.Get(0).(time.Duration)
	}
	timerProvider = timermock.NewMockProvider(ctrl)
	state = New(WithRequestTimeout(requestTimeout), WithTimerProvider(timerProvider))
	return state, timerProvider, handleTimeout
}

func randTimeout() time.Duration {
	return time.Duration(rand.Intn(math.MaxInt32-1) + 1) // positive nonzero
}
