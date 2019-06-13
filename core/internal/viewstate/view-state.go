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
// There are two distinct modes of operation for a replica: 'normal'
// mode and 'view-change' mode.
//
// In the normal mode, a replica operates in a stable configuration,
// called view, represented by the active view number. Initially, a
// replica operates in the normal mode with an active view number
// equal to zero. The active view can only be changed to a higher view
// number via a transition through the view-change mode.
//
// A replica can request to change the view at any time. Requesting a
// view change does not change the mode of operation between normal
// and view-change, but helps peer replicas to coordinate the
// beginning of a view-change operation.
//
// A replica begins a transition to a new view by first switching to
// the view-change mode. Since there is no active view in this mode, a
// replica waits for a new view to become the active one. It will then
// shift to the normal mode as soon as there is a new established
// active view. The new view number can be increased multiple times
// before the view change is complete.
//
// Finally, as stated above, a replica transitions to the new view by
// switching back to the normal mode when the view-change operation is
// complete.
package viewstate

import "sync"

// State defines operations on the state of the view-change mechanism
// maintained by each replica. All methods are safe to invoke
// concurrently.
//
// WaitAndHoldActiveView waits for active view and defers view change.
// If already in the view-change mode, it blocks and waits for the
// normal mode. The returned view number is guaranteed to denote the
// current active view until the returned release function is invoked.
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
// If the supplied view number is greater than the last requested and
// the active view number then it returns true and records the view
// number as last requested; otherwise, false is returned.
//
// StartViewChange synchronizes beginning of view change. If in the
// normal mode and the supplied value is higher than the active view
// number then it switches to the view-change mode with the expected
// new view number equal to the supplied value. If already in the
// view-change mode and the supplied value is higher than the expected
// new view number then the expected new view number is updated.
// Before switching to the view-change mode, this method blocks any
// new invocation of WaitAndHoldActiveView or WaitAndHoldView and
// waits for all release functions returned from WaitAndHoldActiveView
// or WaitAndHoldView to be invoked. It returns true if the replica
// switched to the view-change mode or the expected new view number
// was increased; otherwise false. If true is returned, any new
// invocation of StartViewChange or FinishViewChange is blocked until
// the returned release function is invoked.
//
// FinishViewChange synchronizes completion of view change. If the
// replica is in the normal mode then it first begins a view change,
// same as StartViewChange. If the supplied value equals to the
// expected new view number, it returns true together with a release
// function; otherwise false. If true is returned, any new invocation
// of StartViewChange or FinishViewChange is blocked until the
// returned release function is invoked. Invocation of the release
// function switches the replica back to the normal mode with the
// active view number equal to the supplied value, and unblocks any
// waiting WaitAndHoldActiveView or WaitAndHoldView invocation.
type State interface {
	WaitAndHoldActiveView() (view uint64, release func())
	WaitAndHoldView(view uint64) (ok bool, release func())
	RequestViewChange(newView uint64) bool
	StartViewChange(newView uint64) (ok bool, release func())
	FinishViewChange(newView uint64) (ok bool, release func())
}

// In the normal mode, if lastNewView == lastActiveView. Otherwise, in
// the view-change mode, and lastNewView > lastActiveView.
type viewState struct {
	sync.Mutex

	// Last active view number
	lastActiveView uint64

	// Last requested view change
	lastRequestedView uint64

	// Last started view change
	lastNewView uint64

	// Cond to signal on view change beginning/completion
	viewChangeStartedFinished *sync.Cond

	// Number of active view holders
	nrActiveViewHolders int

	// Cond to signal on active view release
	activeViewReleased *sync.Cond

	// Flag to indicate view change is blocked
	viewChangeBlocked bool

	// Cond to signal when view change is unblocked
	viewChangeUnblocked *sync.Cond
}

// New creates a new instance of the view-change mechanism state.
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

	for !s.inNormalMode() {
		s.viewChangeStartedFinished.Wait()
	}

	s.nrActiveViewHolders++

	return s.lastActiveView, s.releaseActiveView
}

func (s *viewState) WaitAndHoldView(view uint64) (ok bool, release func()) {
	s.Lock()
	defer s.Unlock()

	for !s.inView(view) {
		if !s.isPossibleNewView(view) {
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

	if newView <= s.lastRequestedView {
		return false
	}
	s.lastRequestedView = newView

	return s.isNewView(newView)
}

func (s *viewState) StartViewChange(newView uint64) (ok bool, release func()) {
	s.Lock()
	defer s.Unlock()

	for {
		if !s.isNewViewCandidate(newView) {
			return false, nil
		}

		if s.viewChangeBlocked {
			s.viewChangeUnblocked.Wait()
			continue // recheck
		}
		s.viewChangeBlocked = true

		s.lastNewView = newView
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
		if !s.isPossibleNewView(newView) {
			return false, nil
		}

		if s.viewChangeBlocked {
			s.viewChangeUnblocked.Wait()
			continue // recheck
		}
		s.viewChangeBlocked = true

		// Check if the view change needs to be started
		if s.isNewViewCandidate(newView) {
			s.lastNewView = newView
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

	s.lastActiveView = s.lastNewView
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

func (s *viewState) inNormalMode() bool {
	return s.lastActiveView == s.lastNewView
}

func (s *viewState) inView(view uint64) bool {
	return s.inNormalMode() && view == s.lastActiveView
}

func (s *viewState) isNewView(view uint64) bool {
	return view > s.lastActiveView
}

func (s *viewState) isNewViewCandidate(view uint64) bool {
	return view > s.lastNewView
}

func (s *viewState) isPossibleNewView(view uint64) bool {
	return s.isNewView(view) && view >= s.lastNewView
}
