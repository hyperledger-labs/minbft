// Copyright (c) 2020 NEC Laboratories Europe GmbH.
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
	"sync"

	"github.com/hyperledger-labs/minbft/common/logger"
	"github.com/hyperledger-labs/minbft/core/internal/backofftimer"
	"github.com/hyperledger-labs/minbft/core/internal/viewstate"
)

// viewChangeRequestor requests view change to a new view if needed.
// The return value indicates if the invocation had any effect. It is
// safe to invoke concurrently.
type viewChangeRequestor func(newView uint64) (ok bool)

// viewChangeTimeoutHandler handles view-change timeout expiration.
//
// The supplied parameter denotes the expected new view number.
// It is safe to invoke concurrently.
type viewChangeTimeoutHandler func(view uint64)

// viewChangeTimerStarter starts the view-change timer.
//
// The supplied parameter denotes the expected new view number.
type viewChangeTimerStarter func(view uint64)

// viewChangeTimerStopper stops and resets the view-view change timer.
type viewChangeTimerStopper func()

// makeRequestTimeoutHandler constructs an instance of
// requestTimeoutHandler given the supplied abstractions.
func makeRequestTimeoutHandler(requestViewChange viewChangeRequestor, logger logger.Logger) requestTimeoutHandler {
	return func(view uint64) {
		newView := view + 1

		if requestViewChange(newView) {
			logger.Warningf("Requested view change to view %d due to request timeout", newView)
		}
	}
}

func makeViewChangeTimeoutHandler(requestViewChange viewChangeRequestor, logger logger.Logger) viewChangeTimeoutHandler {
	return func(view uint64) {
		newView := view + 1

		if requestViewChange(newView) {
			logger.Warningf("Requested view change to view %d due to view-change timeout", newView)
		}
	}
}

// makeRequestTimeoutHandler creates an instance of
// viewChangeRequestor using id as local replica identifier and the
// supplied abstractions.
func makeViewChangeRequestor(id uint32, viewState viewstate.State, handleGeneratedMessage generatedMessageHandler) viewChangeRequestor {

	var (
		lock      sync.Mutex
		requested uint64
	)

	return func(newView uint64) (ok bool) {
		lock.Lock()
		defer lock.Unlock()

		if requested >= newView {
			return false
		}
		requested = newView

		_, expectedView, releaseView := viewState.HoldView()
		defer releaseView()

		if expectedView >= newView {
			return false
		}

		handleGeneratedMessage(messageImpl.NewReqViewChange(id, newView))

		return true
	}
}

func makeViewChangeTimerStarter(timer backofftimer.Timer, handleTimeout viewChangeTimeoutHandler, logger logger.Logger) viewChangeTimerStarter {
	return func(view uint64) {
		timer.Start(func() {
			logger.Warningf("View-change timer for expected new view %d expired", view)
			handleTimeout(view)
		})
	}
}
