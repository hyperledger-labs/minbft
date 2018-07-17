// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
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
	"time"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nec-blockchain/minbft/messages"
	"github.com/nec-blockchain/minbft/usig"
)

func TestMakeCommitHandler(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	id := randReplicaID(n)
	view := randView()
	primary := primaryID(n, view)
	provideView := func() uint64 {
		args := mock.MethodCalled("viewProvider")
		return args.Get(0).(uint64)
	}
	mock.On("viewProvider").Return(view)
	acceptUI := func(msg messages.MessageWithUI) (new bool, err error) {
		args := mock.MethodCalled("uiAcceptor", msg)
		return args.Bool(0), args.Error(1)
	}
	handlePrepare := func(prepare *messages.Prepare) (new bool, err error) {
		args := mock.MethodCalled("prepareHandler", prepare)
		return args.Bool(0), args.Error(1)
	}
	collectCommit := func(commit *messages.Commit) error {
		args := mock.MethodCalled("collectCommit", commit)
		return args.Error(0)
	}
	commitUI := func(msg messages.MessageWithUI) {
		mock.MethodCalled("uiCommitter", msg)
	}
	handle := makeCommitHandler(id, n, provideView, acceptUI, handlePrepare, collectCommit, commitUI)

	prepareUIBytes := make([]byte, 1)
	rand.Read(prepareUIBytes)
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: rand.Uint32(),
		},
	}
	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			View:      view,
			ReplicaId: primary,
			Request:   request,
		},
		ReplicaUi: prepareUIBytes,
	}
	makeCommitMsg := func(view uint64) *messages.Commit {
		return &messages.Commit{
			Msg: &messages.Commit_M{
				View:      view,
				ReplicaId: randOtherReplicaID(id, n),
				PrimaryId: primary,
				Request:   request,
				PrimaryUi: prepareUIBytes,
			},
		}
	}

	commit := makeCommitMsg(view)

	mock.On("uiAcceptor", commit).Return(false, fmt.Errorf("Invalid UI")).Once()
	_, err := handle(commit)
	assert.Error(t, err, "UI check failed")

	mock.On("uiAcceptor", commit).Return(false, nil).Once()
	mock.On("prepareHandler", prepare).Return(false, nil).Once()
	mock.On("collectCommit", commit).Return(nil).Once()
	new, err := handle(commit)
	assert.NoError(t, err)
	assert.False(t, new, "UI already processed")

	commit = makeCommitMsg(view + 1)
	mock.On("uiAcceptor", commit).Return(true, nil).Once()
	mock.On("uiCommitter", commit).Once()
	_, err = handle(commit)
	assert.Error(t, err, "Commit is for different view")

	commit = makeCommitMsg(view)

	mock.On("uiAcceptor", commit).Return(true, nil).Once()
	mock.On("uiCommitter", commit).Once()
	mock.On("prepareHandler", prepare).Return(false, fmt.Errorf("Invalid Prepare")).Once()
	_, err = handle(commit)
	assert.Error(t, err, "Commit is for invalid Prepare")

	mock.On("uiAcceptor", commit).Return(true, nil).Once()
	mock.On("uiCommitter", commit).Once()
	mock.On("prepareHandler", prepare).Return(false, nil).Once()
	mock.On("collectCommit", commit).Return(fmt.Errorf("Duplicated Commit")).Once()
	_, err = handle(commit)
	assert.Error(t, err, "Commit cannot be taken into account")

	mock.On("uiAcceptor", commit).Return(true, nil).Once()
	mock.On("uiCommitter", commit).Once()
	mock.On("prepareHandler", prepare).Return(false, nil).Once()
	mock.On("collectCommit", commit).Return(nil).Once()
	new, err = handle(commit)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeCommitCollector(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	counter := func(commit *messages.Commit) (done bool, err error) {
		args := mock.MethodCalled("commitCounter", commit)
		return args.Bool(0), args.Error(1)
	}
	executor := func(request *messages.Request) {
		mock.MethodCalled("requestExecutor", request)
	}

	collector := makeCommitCollector(counter, executor)

	clientID := rand.Uint32()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: clientID,
		},
	}
	commit := &messages.Commit{
		Msg: &messages.Commit_M{
			Request: request,
		},
	}

	mock.On("commitCounter", commit).Return(false, fmt.Errorf("duplicate commit")).Once()
	err := collector(commit)
	assert.Error(t, err)

	mock.On("commitCounter", commit).Return(false, nil).Once()
	err = collector(commit)
	assert.NoError(t, err)

	requestExecuted := make(chan time.Time)
	mock.On("commitCounter", commit).Return(true, nil).Once()
	mock.On("requestExecutor", request).WaitUntil(requestExecuted).Once()
	err = collector(commit)
	assert.NoError(t, err)
	requestExecuted <- time.Time{}
}

func TestMakeCommitCounter(t *testing.T) {
	// fault tolerance -> list of cases
	cases := map[uint32][]struct {
		desc      string
		prepareCV int
		replicaID int
		ok        bool
		done      bool
	}{
		// f=1
		1: {{
			// Commit from primary replica is implied by
			// extracted Prepare
			desc:      "One Commit from backup replica is enough",
			prepareCV: 1,
			replicaID: 1,
			ok:        true,
			done:      true,
		}, {
			desc:      "Extra Commit from another backup replica is ignored",
			prepareCV: 1,
			replicaID: 2,
			ok:        true,
		}, {
			desc:      "Commit from primary is not okay",
			prepareCV: 2, // new Prepare
			replicaID: 0, // primary is always replica 0 for this test
			ok:        false,
		}},

		// f=2
		2: {{
			desc:      "First Commit from backup replica",
			prepareCV: 1,
			replicaID: 1,
			ok:        true,
		}, {
			desc:      "Another Commit for another Prepare",
			prepareCV: 2,
			replicaID: 1,
			ok:        true,
		}, {
			desc:      "Duplicate Commit is not okay",
			prepareCV: 1,
			replicaID: 1,
			ok:        false,
		}, {
			desc:      "Another Commit from backup replica is enough",
			prepareCV: 1,
			replicaID: 3,
			ok:        true,
			done:      true,
		}, {
			desc:      "The second Prepared request is done",
			prepareCV: 2,
			replicaID: 2,
			ok:        true,
			done:      true,
		}, {
			desc:      "Extra Commit is ingnored",
			prepareCV: 1,
			replicaID: 2,
			ok:        true,
		}},
	}

	for f, caseList := range cases {
		counter := makeCommitCounter(f)
		for _, c := range caseList {
			desc := fmt.Sprintf("f=%d: %s", f, c.desc)
			done, err := counter(makeCommit(c.prepareCV, c.replicaID))
			if c.ok {
				require.NoError(t, err, desc)
			} else {
				require.Error(t, err, desc)
			}
			require.Equal(t, c.done, done, desc)
		}
	}
}

func TestMakeCommitCounterConcurrent(t *testing.T) {
	const nrFaulty = 2
	const nrReplicas = 2*nrFaulty + 1
	const nrPrepares = 100

	wg := new(sync.WaitGroup)

	counter := makeCommitCounter(nrFaulty)
	for id := 1; id < nrReplicas; id++ { // replica 0 is primary
		wg.Add(1)
		go func(replicaID int) {
			defer wg.Done()

			for prepareCV := 1; prepareCV <= nrPrepares; prepareCV++ {
				// We can't check how many times the
				// counter was invoked before
				// signaling done and still invoke it
				// concurrently. So we only check for
				// data races here.
				_, err := counter(makeCommit(prepareCV, replicaID))
				assert.NoError(t, err,
					"Replica %d, Prepare %d", replicaID, prepareCV)
			}
		}(id)
	}

	wg.Wait()
}

func makeCommit(prepareCV, replicaID int) *messages.Commit {
	prepareUI := &usig.UI{
		Counter: uint64(prepareCV),
	}
	prepareUIBytes, _ := prepareUI.MarshalBinary()

	return &messages.Commit{
		Msg: &messages.Commit_M{
			ReplicaId: uint32(replicaID),
			PrimaryUi: prepareUIBytes,
		},
	}
}
