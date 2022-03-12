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
// There are two main values that represent the state of the view:
// 'current' and 'expected' view numbers. The values initially equal
// to zero and can only monotonically increase.
package viewstate

import "sync"

// State defines operations on view state. All methods are safe to
// invoke concurrently.
//
// HoldView method returns current and expected view numbers and
// defers view change. The returned values will denote the actual view
// state until the returned release function is invoked.
//
// AdvanceExpectedView method synchronizes beginning of view change.
// If the supplied view number is greater than the expected view
// number then the latter is increased to match the supplied one. It
// returns true if the expected view number was updated. In that case,
// the current and expected view numbers will remain the same until
// the returned release function is invoked.
//
// AdvanceCurrentView method synchronizes transition into a new view.
// If the supplied value is greater than the current view number then
// the latter is increased to match the supplied one. It returns true
// if the current view number was updated. In that case, the current
// and expected view numbers will remain the same, and the returned
// expected view number will denote the actual view state, until the
// returned release function is invoked.
type State interface {
	HoldView() (current, expected uint64, release func())
	AdvanceExpectedView(view uint64) (ok bool, release func())
	AdvanceCurrentView(view uint64) (ok bool, expected uint64, release func())
}

type viewState struct {
	sync.RWMutex

	currentView  uint64
	expectedView uint64
}

// New creates a new instance of the view state.
func New() State {
	return &viewState{}
}

func (s *viewState) HoldView() (current, expected uint64, release func()) {
	s.RLock()
	release = s.RUnlock

	return s.currentView, s.expectedView, release
}

func (s *viewState) AdvanceExpectedView(view uint64) (ok bool, release func()) {
	s.Lock()
	release = s.Unlock

	if s.expectedView < view {
		s.expectedView = view

		return true, release
	}

	release()
	return false, nil
}

func (s *viewState) AdvanceCurrentView(view uint64) (ok bool, expected uint64, release func()) {
	s.Lock()
	release = s.Unlock

	if s.currentView < view {
		s.currentView = view

		return true, s.expectedView, release
	}

	release()
	return false, 0, nil
}
