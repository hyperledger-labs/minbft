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

package viewstate

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewState(t *testing.T) {
	s := New()

	cases := []struct {
		desc string

		view int

		request bool
		start   bool
		finish  bool

		wait   bool
		active bool

		ok bool
	}{{
		desc:   "Wait for view #0",
		view:   0,
		wait:   true,
		active: true,
		ok:     true,
	}, {
		desc:    "Request, begin and complete change to view #0",
		view:    0,
		request: true,
		start:   true,
		finish:  true,
		ok:      false,
	}, {
		desc:    "Request change to view #1",
		view:    1,
		request: true,
		ok:      true,
	}, {
		desc:    "Request change to view #1 again",
		view:    1,
		request: true,
		ok:      false,
	}, {
		desc:   "Begin and complete change to view #1",
		view:   1,
		start:  true,
		finish: true,
		wait:   true,
		active: true,
		ok:     true,
	}, {
		desc:   "Begin and complete change to view #1 again",
		view:   1,
		start:  true,
		finish: true,
		ok:     false,
	}, {
		desc:    "Request and begin change to view #2",
		view:    2,
		request: true,
		start:   true,
		ok:      true,
	}, {
		desc: "Wait for view #1",
		view: 1,
		wait: true,
		ok:   false,
	}, {
		desc:    "Request and begin change to view #3",
		view:    3,
		request: true,
		start:   true,
		ok:      true,
	}, {
		desc:   "Complete change to view #2",
		view:   2,
		finish: true,
		ok:     false,
	}, {
		desc: "Wait for view #2",
		view: 2,
		wait: true,
		ok:   false,
	}, {
		desc:    "Request and begin change to view #3 again",
		view:    3,
		request: true,
		start:   true,
		ok:      false,
	}, {
		desc:   "Complete change to view #3",
		view:   3,
		finish: true,
		wait:   true,
		active: true,
		ok:     true,
	}, {
		desc:   "Complete change to view #4",
		view:   4,
		finish: true,
		wait:   true,
		active: true,
		ok:     true,
	}, {
		desc:    "Request and begin change to view #4",
		view:    4,
		request: true,
		start:   true,
		ok:      false,
	}}

	for _, c := range cases {
		view := uint64(c.view)
		if c.request {
			ok := s.RequestViewChange(view)
			require.Equal(t, c.ok, ok, c.desc)
		}
		if c.start {
			ok, release := s.StartViewChange(view)
			require.Equal(t, c.ok, ok, c.desc)
			if ok {
				release()
			}
		}
		if c.finish {
			ok, release := s.FinishViewChange(view)
			require.Equal(t, c.ok, ok, c.desc)
			if ok {
				release()
			}
		}
		if c.wait {
			ok, release := s.WaitAndHoldView(view)
			require.Equal(t, c.ok, ok, c.desc)
			if ok {
				release()
			}
		}
		if c.active {
			activeView, release := s.WaitAndHoldActiveView()
			require.Equal(t, view, activeView, c.desc)
			release()
		}
	}
}

func TestConcurrent(t *testing.T) {
	const nrConcurrent = 7
	const nrViewChanges = 17

	wg := new(sync.WaitGroup)

	runConcurrently := func(f func()) {
		wg.Add(1)

		go func() {
			f()
			wg.Done()
		}()
	}

	var (
		active    uint64
		requested uint64
		started   uint64
		finished  uint64
	)

	s := New()

	assertActiveView := func(view uint64) {
		lastActive := atomic.SwapUint64(&active, view)
		assert.True(t, lastActive <= view,
			"Active view number cannot decrease")
		assert.True(t, started <= view,
			"View change cannot begin before view released")
		assert.True(t, finished == view,
			"View change cannot complete before view released")
	}

	waitAndHoldActiveView := func(view uint64) {
		for {
			activeView, release := s.WaitAndHoldActiveView()
			assertActiveView(activeView)
			go release()
			if activeView >= view {
				break
			}
		}
	}

	waitAndHoldView := func(view uint64) {
		ok, release := s.WaitAndHoldView(view)
		if !ok {
			return
		}
		assertActiveView(view)
		go release()
	}

	requestViewChange := func(view uint64) {
		lastActive := atomic.LoadUint64(&active)
		lastRequested := atomic.LoadUint64(&requested)
		lastFinished := atomic.LoadUint64(&finished)

		if !s.RequestViewChange(view) {
			return
		}
		atomic.CompareAndSwapUint64(&requested, lastRequested, view)
		assert.True(t, lastRequested < view,
			"Requested view change number cannot decrease")
		assert.True(t, lastActive < view,
			"Requested view change number must be higher than last active")
		assert.True(t, lastFinished < view,
			"Requested view change number must be higher than last finished")

	}

	startViewChange := func(view uint64) {
		ok, release := s.StartViewChange(view)
		if !ok {
			return
		}
		assert.True(t, started < view,
			"Started view change number cannot decrease")
		started = view
		assert.True(t, finished < view,
			"Started view change number must be higher than last finished")
		assert.True(t, active < view,
			"Started view change number must be higher than last active")
		go release()
	}

	finishViewChange := func(view uint64) {
		ok, release := s.FinishViewChange(view)
		if !ok {
			return
		}
		assert.True(t, finished < view,
			"Finished view change number cannot decrease")
		atomic.StoreUint64(&finished, view)
		assert.True(t, started <= view,
			"Finished view change number cannot be lower than last started")
		assert.True(t, active < view,
			"Finished view change number must be higher than last active")
		go release()
	}

	for i := 0; i < nrConcurrent; i++ {
		for view := uint64(0); view <= nrViewChanges; view++ {
			view := view
			runConcurrently(func() { waitAndHoldActiveView(view) })
			runConcurrently(func() { waitAndHoldView(view) })
			runConcurrently(func() { requestViewChange(view) })
			runConcurrently(func() { startViewChange(view) })
			runConcurrently(func() { finishViewChange(view) })
		}
	}

	wg.Wait()
}
