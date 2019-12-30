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
// WaitAndHoldActiveView waits for active view and defers view change.
// It waits until current and expected view numbers become equal, then
// returns the current active view number. The returned view number is
// guaranteed to denote the current active view until the returned
// release function is invoked.
//
// WaitAndHoldView waits for specific active view and defers view
// change. This method resembles WaitAndHoldActiveView, but only waits
// for a specific active view number supplied through the parameter
// view. It immediately returns false once it is detected that the
// supplied view number will never become active. Otherwise, it
// returns true as soon as the awaited view number is active. In that
// case, the view will stay active until the returned release function
// is invoked.
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
	WaitAndHoldActiveView() (view uint64, release func())
	WaitAndHoldView(view uint64) (ok bool, release func())
	RequestViewChange(newView uint64) bool
	StartViewChange(newView uint64) (ok bool, release func())
	FinishViewChange(newView uint64) (ok bool, release func())
}

type viewState struct {
	sync.Mutex

	currentView   uint64
	expectedView  uint64
	requestedView uint64

	// Cond to signal on updating current/expected view
	viewChangeStartedFinished *sync.Cond

	// Number of active view holders
	nrActiveViewHolders int

	// Cond to signal on active view release
	activeViewReleased *sync.Cond

	// Flag to block updating current/expected view
	viewChangeBlocked bool

	// Cond to signal unblocking current/expected view
	viewChangeUnblocked *sync.Cond
}

// New creates a new instance of the view state.
func New() State {
	s := new(viewState)
	s.viewChangeStartedFinished = sync.NewCond(s)
	s.activeViewReleased = sync.NewCond(s)
	s.viewChangeUnblocked = sync.NewCond(s)

	return s
}

func (s *viewState) WaitAndHoldActiveView() (view uint64, release func()) {
	s.Lock()
	defer s.Unlock()

	for s.currentView != s.expectedView {
		s.viewChangeStartedFinished.Wait()
	}

	s.nrActiveViewHolders++

	return s.currentView, s.releaseActiveView
}

func (s *viewState) WaitAndHoldView(view uint64) (ok bool, release func()) {
	s.Lock()
	defer s.Unlock()

	for view != s.currentView || s.currentView != s.expectedView {
		if view < s.currentView || view < s.expectedView {
			return false, nil
		}

		s.viewChangeStartedFinished.Wait()
	}

	s.nrActiveViewHolders++

	return true, s.releaseActiveView
}

func (s *viewState) releaseActiveView() {
	s.Lock()
	defer s.Unlock()

	s.nrActiveViewHolders--
	if s.nrActiveViewHolders == 0 {
		s.activeViewReleased.Broadcast()
	} else if s.nrActiveViewHolders < 0 {
		panic("Duplicate active view release")
	}
}

func (s *viewState) RequestViewChange(newView uint64) bool {
	s.Lock()
	defer s.Unlock()

	if newView <= s.requestedView {
		return false
	}
	s.requestedView = newView

	return newView > s.currentView
}

func (s *viewState) StartViewChange(newView uint64) (ok bool, release func()) {
	s.Lock()
	defer s.Unlock()

	for {
		if newView <= s.expectedView {
			return false, nil
		}

		if s.viewChangeBlocked {
			s.viewChangeUnblocked.Wait()
			continue // recheck
		}
		s.viewChangeBlocked = true

		s.expectedView = newView
		s.viewChangeStartedFinished.Broadcast()

		for s.nrActiveViewHolders > 0 {
			s.activeViewReleased.Wait()
		}

		return true, s.unblockViewChange
	}
}

func (s *viewState) FinishViewChange(newView uint64) (ok bool, release func()) {
	s.Lock()
	defer s.Unlock()

	for {
		if newView <= s.currentView || newView < s.expectedView {
			return false, nil
		}

		if s.viewChangeBlocked {
			s.viewChangeUnblocked.Wait()
			continue // recheck
		}
		s.viewChangeBlocked = true

		// Check if the expected view needs to be updated
		if s.expectedView < newView {
			s.expectedView = newView
			s.viewChangeStartedFinished.Broadcast()
		}

		for s.nrActiveViewHolders > 0 {
			s.activeViewReleased.Wait()
		}

		return true, s.finishAndUnblockViewChange
	}
}

func (s *viewState) finishAndUnblockViewChange() {
	s.Lock()
	defer s.Unlock()

	s.currentView = s.expectedView
	s.viewChangeStartedFinished.Broadcast()
	s.unblockViewChangeLocked()
}

func (s *viewState) unblockViewChange() {
	s.Lock()
	defer s.Unlock()

	s.unblockViewChangeLocked()
}

func (s *viewState) unblockViewChangeLocked() {
	s.viewChangeBlocked = false
	s.viewChangeUnblocked.Broadcast()
}
