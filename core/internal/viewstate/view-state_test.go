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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yaml "gopkg.in/yaml.v2"
)

func TestViewState(t *testing.T) {
	s := New()

	var cases []struct {
		View int

		Request bool
		Start   bool
		Finish  bool

		Current  int
		Expected int

		Ok bool
	}
	casesYAML := []byte(`
- {view: 0, request: y, start: y, finish: y, current: 0, expected: 0, ok: n}
- {view: 1, request: y, start: n, finish: n, current: 0, expected: 0, ok: y}
- {view: 1, request: y, start: n, finish: n, current: 0, expected: 0, ok: n}
- {view: 1, request: n, start: y, finish: y, current: 1, expected: 1, ok: y}
- {view: 1, request: n, start: y, finish: y, current: 1, expected: 1, ok: n}
- {view: 2, request: y, start: y, finish: n, current: 1, expected: 2, ok: y}
- {view: 3, request: y, start: y, finish: n, current: 1, expected: 3, ok: y}
- {view: 2, request: n, start: n, finish: y, current: 1, expected: 3, ok: n}
- {view: 3, request: y, start: y, finish: n, current: 1, expected: 3, ok: n}
- {view: 3, request: n, start: n, finish: y, current: 3, expected: 3, ok: y}
- {view: 4, request: n, start: n, finish: y, current: 4, expected: 4, ok: y}
- {view: 4, request: y, start: y, finish: n, current: 4, expected: 4, ok: n}
`)
	if err := yaml.UnmarshalStrict(casesYAML, &cases); err != nil {
		t.Fatal(err)
	}

	for i, c := range cases {
		assertMsg := fmt.Sprintf("Case #%d", i)
		view := uint64(c.View)
		if c.Request {
			ok := s.RequestViewChange(view)
			require.Equal(t, c.Ok, ok, assertMsg)
		}
		if c.Start {
			ok, release := s.StartViewChange(view)
			require.Equal(t, c.Ok, ok, assertMsg)
			if ok {
				release()
			}
		}
		if c.Finish {
			ok, release := s.FinishViewChange(view)
			require.Equal(t, c.Ok, ok, assertMsg)
			if ok {
				release()
			}
		}
		current, expected, release := s.HoldView()
		require.EqualValues(t, c.Current, current, assertMsg)
		require.EqualValues(t, c.Expected, expected, assertMsg)
		release()
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
		requested uint64
		started   uint64
		finished  uint64
	)

	s := New()

	holdView := func(view uint64) {
		for {
			current, expected, release := s.HoldView()
			assert.True(t, current <= expected)
			if current == expected {
				assert.True(t, current == finished)
			} else if current < expected {
				assert.True(t, expected == started)
			}
			go release()
			if current >= view {
				break
			}
		}
	}

	requestViewChange := func(view uint64) {
		lastRequested := atomic.LoadUint64(&requested)
		lastFinished := atomic.LoadUint64(&finished)

		if !s.RequestViewChange(view) {
			return
		}
		atomic.CompareAndSwapUint64(&requested, lastRequested, view)
		assert.True(t, lastRequested < view,
			"Requested view change number cannot decrease")
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
		go release()
	}

	for i := 0; i < nrConcurrent; i++ {
		for view := uint64(0); view <= nrViewChanges; view++ {
			view := view
			runConcurrently(func() { holdView(view) })
			runConcurrently(func() { requestViewChange(view) })
			runConcurrently(func() { startViewChange(view) })
			runConcurrently(func() { finishViewChange(view) })
		}
	}

	wg.Wait()
}
