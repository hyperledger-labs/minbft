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

// Package viewstate allows to manage current active view number. It
// provides means for obtaining the current active view number, as
// well as to coordinate the process of changing its value according
// to the view-change mechanism.
//
// There are three main values that represent the state of the view:
// 'current', 'expected', and 'requested' view numbers. The values
// initially equal to zero and can only monotonically increase. The
// current view is active if current and expected view numbers match.
package viewstate

import "sync"

// State defines operations on view state. All methods are safe to
// invoke concurrently.
//
// HoldView method returns current and expected view numbers and
// defers view change. The returned values will denote the actual view
// state until the returned release function is invoked.
//
// RequestViewChange handles synchronization of view change requests.
// If the supplied value is greater than both the requested and
// current view numbers then the requested view number is increased to
// match the supplied value. It returns true if the requested view
// number was updated.
//
// StartViewChange synchronizes beginning of view change. If the
// supplied view number is greater than the expected view number then
// the latter is increased to match the supplied one. It returns true
// if the expected view number was updated. In that case, the current
// and expected view numbers will remain the same until the returned
// release function is invoked.
//
// FinishViewChange synchronizes completion of view change. First, it
// attempts to increase the expected view number to match the supplied
// view number, same as StartViewChange. After that, if the supplied
// value is greater than the current view number then the latter is
// increased to match the supplied one. It returns true if the current
// view number was updated and matches the expected view number. In
// that case, the current and expected view numbers will remain the
// same until the returned release function is invoked.
type State interface {
	HoldView() (current, expected uint64, release func())
	RequestViewChange(newView uint64) bool
	StartViewChange(newView uint64) (ok bool, release func())
	FinishViewChange(newView uint64) (ok bool, release func())
}

type viewState struct {
	currentExpectedViewLock sync.RWMutex
	requestedViewLock       sync.Mutex

	currentView   uint64
	expectedView  uint64
	requestedView uint64
}

// New creates a new instance of the view state.
func New() State {
	return &viewState{}
}

func (s *viewState) HoldView() (current, expected uint64, release func()) {
	s.currentExpectedViewLock.RLock()
	release = s.currentExpectedViewLock.RUnlock

	return s.currentView, s.expectedView, release
}

func (s *viewState) RequestViewChange(newView uint64) bool {
	s.requestedViewLock.Lock()
	defer s.requestedViewLock.Unlock()

	if newView <= s.requestedView {
		return false
	}
	s.requestedView = newView

	s.currentExpectedViewLock.RLock()
	defer s.currentExpectedViewLock.RUnlock()

	return newView > s.currentView
}

func (s *viewState) StartViewChange(newView uint64) (ok bool, release func()) {
	s.currentExpectedViewLock.Lock()
	release = s.currentExpectedViewLock.Unlock

	if s.expectedView < newView {
		s.expectedView = newView

		return true, release
	}

	release()
	return false, nil
}

func (s *viewState) FinishViewChange(newView uint64) (ok bool, release func()) {
	s.currentExpectedViewLock.Lock()
	release = s.currentExpectedViewLock.Unlock

	if s.currentView < newView && s.expectedView <= newView {
		// Check if the expected view needs to be updated
		if s.expectedView < newView {
			s.expectedView = newView
		}

		s.currentView = s.expectedView

		return true, release
	}

	release()
	return false, nil
}
