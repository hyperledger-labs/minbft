// Copyright (c) 2022 NEC Laboratories Europe GmbH.
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

package backofftimer

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	timermock "github.com/hyperledger-labs/minbft/core/internal/timer/mock"
	testifymock "github.com/stretchr/testify/mock"

	timerImpl "github.com/hyperledger-labs/minbft/core/internal/timer"
)

func TestTimer(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handleTimeout := func() {
		mock.MethodCalled("timeoutHandler")
	}
	mockTimer := timermock.NewMockTimer(ctrl)

	initTimeout := randTimeout()
	timerProvider := timermock.NewMockProvider(ctrl)
	timer := New(initTimeout, WithTimerProvider(timerProvider))

	var expectedTimeout time.Duration
	timerProvider.EXPECT().AfterFunc(gomock.Any(), gomock.Any()).DoAndReturn(
		func(d time.Duration, f func()) timerImpl.Timer {
			assert.Equal(t, expectedTimeout, d)
			f()
			return mockTimer
		},
	).AnyTimes()

	expectedTimeout = initTimeout
	mock.On("timeoutHandler").Once()
	timer.Start(handleTimeout)

	expectedTimeout *= 2
	mock.On("timeoutHandler").Once()
	timer.Start(handleTimeout)

	mockTimer.EXPECT().Stop()
	timer.Stop()

	expectedTimeout = initTimeout
	mock.On("timeoutHandler").Once()
	timer.Start(handleTimeout)

	expectedTimeout *= 2
	mock.On("timeoutHandler").Once()
	timer.Start(handleTimeout)
}

func randTimeout() time.Duration {
	return time.Duration(rand.Intn(math.MaxInt32-1) + 1) // positive nonzero
}
