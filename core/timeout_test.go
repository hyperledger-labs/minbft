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
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	testifymock "github.com/stretchr/testify/mock"
	yaml "gopkg.in/yaml.v2"

	"github.com/hyperledger-labs/minbft/common/logger"
	"github.com/hyperledger-labs/minbft/core/internal/viewstate"
	"github.com/hyperledger-labs/minbft/messages"

	mock_viewstate "github.com/hyperledger-labs/minbft/core/internal/viewstate/mocks"
)

func TestMakeRequestTimeoutHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	requestViewChange := func(nv uint64) (ok bool) {
		args := mock.MethodCalled("viewChangeRequestor", nv)
		return args.Bool(0)
	}

	handle := makeRequestTimeoutHandler(requestViewChange, logger.NewReplicaLogger(0))

	view := rand.Uint64()

	mock.On("viewChangeRequestor", view+1).Return(false).Once()
	handle(view)
}

func TestMakeViewChangeRequestor(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var cases []struct {
		View     uint64
		Expected uint64
		Ok       bool
	}

	casesYAML := []byte(`
- {view: 0, expected: 0, ok: n}
- {view: 1, expected: 0, ok: y}
- {view: 1, expected: 0, ok: n}
- {view: 2, expected: 2, ok: n}
- {view: 3, expected: 2, ok: y}
`)

	if err := yaml.UnmarshalStrict(casesYAML, &cases); err != nil {
		t.Fatal(err)
	}

	id := rand.Uint32()

	var expectedView uint64
	viewState := mock_viewstate.NewMockState(ctrl)
	viewState.EXPECT().HoldView().DoAndReturn(func() (uint64, uint64, func()) {
		mock.On("viewReleaser").Once()
		return 0, expectedView, func() {
			mock.MethodCalled("viewReleaser")
		}
	}).AnyTimes()
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}

	request := makeViewChangeRequestor(id, viewState, handleGeneratedMessage)

	for i, c := range cases {
		assertMsg := fmt.Sprintf("Case #%d", i)
		expectedView = c.Expected
		if c.Ok {
			msg := messageImpl.NewReqViewChange(id, c.View)
			mock.On("generatedMessageHandler", msg).Once()
		}
		ok := request(c.View)
		assert.Equal(t, c.Ok, ok, assertMsg)
	}
}

func TestMakeViewChangeRequestorConcurrent(t *testing.T) {
	const nrViews = 100
	const nrConcurrent = 10

	var msgs []messages.ReqViewChange

	viewState := viewstate.New()
	consumeMessage := func(m messages.ReplicaMessage) {
		msgs = append(msgs, m.(messages.ReqViewChange))
	}
	request := makeViewChangeRequestor(rand.Uint32(), viewState, consumeMessage)

	wg := new(sync.WaitGroup)
	defer wg.Wait()

	for v := 0; v < nrViews; v++ {
		v := uint64(v)

		wg.Add(nrConcurrent)
		for i := 0; i < nrConcurrent; i++ {
			go func() {
				_ = request(v)
				wg.Done()
			}()
		}

		if ok, releaseView := viewState.AdvanceExpectedView(v); ok {
			if len(msgs) == 1 {
				assert.Equal(t, v, msgs[0].NewView())
			} else {
				assert.Empty(t, msgs)
			}
			msgs = nil
			releaseView()
		}
	}
}
