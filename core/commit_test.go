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

	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
)

func TestMakeCommitValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	backup := randOtherReplicaID(primary, n)

	verifyUI := func(msg messages.MessageWithUI) (*usig.UI, error) {
		args := mock.MethodCalled("uiVerifier", msg)
		return args.Get(0).(*usig.UI), args.Error(1)
	}
	validatePrepare := func(prepare *messages.Prepare) error {
		args := mock.MethodCalled("prepareValidator", prepare)
		return args.Error(0)
	}
	validate := makeCommitValidator(verifyUI, validatePrepare)

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
	}
	ui := &usig.UI{Counter: rand.Uint64()}
	makeCommitMsg := func(id uint32) *messages.Commit {
		return &messages.Commit{
			Msg: &messages.Commit_M{
				View:      view,
				ReplicaId: id,
				PrimaryId: primary,
				Request:   request,
			},
		}
	}

	commit := makeCommitMsg(primary)
	err := validate(commit)
	assert.Error(t, err, "Commit from primary")

	commit = makeCommitMsg(backup)

	mock.On("prepareValidator", prepare).Return(fmt.Errorf("UI not valid")).Once()
	err = validate(commit)
	assert.Error(t, err, "Invalid Prepare")

	mock.On("prepareValidator", prepare).Return(nil).Once()
	mock.On("uiVerifier", commit).Return((*usig.UI)(nil), fmt.Errorf("UI not valid")).Once()
	err = validate(commit)
	assert.Error(t, err, "Invalid UI")

	mock.On("prepareValidator", prepare).Return(nil).Once()
	mock.On("uiVerifier", commit).Return(ui, nil).Once()
	err = validate(commit)
	assert.NoError(t, err)
}

func TestMakeCommitApplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	collectCommitment := func(id uint32, prepare *messages.Prepare) error {
		args := mock.MethodCalled("commitmentCollector", id, prepare)
		return args.Error(0)
	}
	apply := makeCommitApplier(collectCommitment)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	id := randOtherReplicaID(primary, n)

	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			ReplicaId: primary,
			View:      view,
		},
	}
	commit := &messages.Commit{
		Msg: &messages.Commit_M{
			View:      view,
			ReplicaId: id,
			PrimaryId: primary,
		},
	}

	mock.On("commitmentCollector", id, prepare).Return(fmt.Errorf("Error")).Once()
	err := apply(commit)
	assert.Error(t, err, "Failed to collect commitment")

	mock.On("commitmentCollector", id, prepare).Return(nil).Once()
	err = apply(commit)
	assert.NoError(t, err)
}

func TestMakeCommitmentCollector(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	countCommitment := func(id uint32, prepare *messages.Prepare) (done bool, err error) {
		args := mock.MethodCalled("commitmentCounter", id, prepare)
		return args.Bool(0), args.Error(1)
	}
	retireSeq := func(request *messages.Request) (new bool) {
		args := mock.MethodCalled("requestSeqRetirer", request)
		return args.Bool(0)
	}
	stopReqTimer := func(clientID uint32) {
		mock.MethodCalled("requestTimerStopper", clientID)
	}
	executeRequest := func(request *messages.Request) {
		mock.MethodCalled("requestExecutor", request)
	}
	collect := makeCommitmentCollector(countCommitment, retireSeq, stopReqTimer, executeRequest)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	id := randOtherReplicaID(primary, n)
	clientID := rand.Uint32()
	request := &messages.Request{
		Msg: &messages.Request_M{
			ClientId: clientID,
			Seq:      rand.Uint64(),
		},
	}
	prepare := &messages.Prepare{
		Msg: &messages.Prepare_M{
			ReplicaId: primary,
			Request:   request,
		},
	}

	mock.On("commitmentCounter", id, prepare).Return(false, fmt.Errorf("Error")).Once()
	err := collect(id, prepare)
	assert.Error(t, err, "Failed to count commitment")

	mock.On("commitmentCounter", id, prepare).Return(false, nil).Once()
	err = collect(id, prepare)
	assert.NoError(t, err)

	mock.On("commitmentCounter", id, prepare).Return(true, nil).Once()
	mock.On("requestSeqRetirer", request).Return(false).Once()
	err = collect(id, prepare)
	assert.NoError(t, err)

	mock.On("commitmentCounter", id, prepare).Return(true, nil).Once()
	mock.On("requestSeqRetirer", request).Return(true).Once()
	mock.On("requestTimerStopper", clientID).Once()
	mock.On("requestExecutor", request).Once()
	err = collect(id, prepare)
	assert.NoError(t, err)
}

func TestMakeCommitmentCollectorConcurrent(t *testing.T) {
	const nrFaulty = 1
	const nrReplicas = 100
	const nrPrepares = 100

	var executedReqs []*messages.Request

	clientStates := clientstate.NewProvider()
	captureSeq := makeRequestSeqCapturer(clientStates)
	prepareSeq := makeRequestSeqPreparer(clientStates)
	retireSeq := makeRequestSeqRetirer(clientStates)
	stopReqTimer := makeRequestTimerStopper(clientStates)
	countCommitment := makeCommitmentCounter(nrFaulty)
	executeRequest := func(req *messages.Request) {
		time.Sleep(time.Millisecond)
		executedReqs = append(executedReqs, req)
	}
	collect := makeCommitmentCollector(countCommitment, retireSeq, stopReqTimer, executeRequest)

	wg := new(sync.WaitGroup)
	for id := 0; id < nrReplicas; id++ {
		id := id

		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i <= nrPrepares; i++ {
				cv := uint64(i + 1)
				seq := cv

				request := &messages.Request{
					Msg: &messages.Request_M{
						Seq: seq,
					},
				}
				if ok, releaseSeq := captureSeq(request); ok {
					releaseSeq()
				}

				prepareUI := &usig.UI{
					Counter: cv,
				}
				prepareUIBytes, _ := prepareUI.MarshalBinary()
				prepare := &messages.Prepare{
					Msg: &messages.Prepare_M{
						Request: request,
					},
					ReplicaUi: prepareUIBytes,
				}
				_ = prepareSeq(request)

				err := collect(uint32(id), prepare)
				assert.NoError(t, err, "Replica %d, Prepare %d", id, cv)
			}
		}()
	}

	wg.Wait()

	for i, req := range executedReqs {
		assert.Equal(t, uint64(i+1), req.Msg.Seq)
	}
}

func TestMakeCommitmentCounter(t *testing.T) {
	// fault tolerance -> list of cases
	cases := map[int][]struct {
		desc      string
		view      int
		prepareCV int
		replicaID int
		ok        bool
		done      bool
	}{
		// f=1
		1: {{
			desc:      "Commitment from primary",
			prepareCV: 1,
			replicaID: 0,
			ok:        true,
			done:      false,
		}, {
			desc:      "One commitment from backup replica is enough",
			prepareCV: 1,
			replicaID: 1,
			ok:        true,
			done:      true,
		}, {
			desc:      "Extra commitment from another backup replica",
			prepareCV: 1,
			replicaID: 2,
			ok:        true,
			done:      true,
		}, {
			desc:      "Second commitment from primary",
			prepareCV: 2,
			replicaID: 0,
			ok:        true,
			done:      false,
		}, {
			desc:      "Third commitment from primary",
			prepareCV: 3,
			replicaID: 0,
			ok:        true,
			done:      false,
		}, {
			desc:      "Non-sequential commitment from backup replica",
			prepareCV: 3,
			replicaID: 2,
			ok:        false,
			done:      false,
		}, {
			desc:      "First commitment in a new view",
			view:      1,
			prepareCV: 2,
			replicaID: 1,
			ok:        true,
			done:      false,
		}, {
			desc:      "Second commitment in a new view",
			view:      1,
			prepareCV: 3,
			replicaID: 1,
			ok:        true,
			done:      false,
		}, {
			desc:      "Commitment for old view",
			view:      0,
			prepareCV: 2,
			replicaID: 2,
			ok:        true,
			done:      false,
		}, {
			desc:      "Non-sequential commitment in a new view",
			view:      1,
			prepareCV: 3,
			replicaID: 0,
			ok:        false,
		}, {
			desc:      "First valid commitment from backup in a new view",
			view:      1,
			prepareCV: 2,
			replicaID: 2,
			ok:        true,
			done:      true,
		}},

		// f=2
		2: {{
			desc:      "Commitment from primary",
			prepareCV: 1,
			replicaID: 0,
			ok:        true,
			done:      false,
		}, {
			desc:      "First commitment from backup replica",
			prepareCV: 1,
			replicaID: 1,
			ok:        true,
		}, {
			desc:      "Another commitment from primary",
			prepareCV: 2,
			replicaID: 0,
			ok:        true,
			done:      false,
		}, {
			desc:      "Another commitment for another Prepare",
			prepareCV: 2,
			replicaID: 1,
			ok:        true,
		}, {
			desc:      "Duplicate commitment is not okay",
			prepareCV: 1,
			replicaID: 1,
			ok:        false,
		}, {
			desc:      "Another commitment from backup replica is enough",
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
			desc:      "Extra commitment for the first request",
			prepareCV: 1,
			replicaID: 2,
			ok:        true,
			done:      true,
		}},
	}

	for f, caseList := range cases {
		n := 2*f + 1
		counter := makeCommitmentCounter(uint32(f))
		for _, c := range caseList {
			desc := fmt.Sprintf("f=%d: %s", f, c.desc)
			v := c.view
			p := v % n
			cv := c.prepareCV
			done, err := counter(uint32(c.replicaID), makePrepare(p, v, cv))
			if c.ok {
				require.NoError(t, err, desc)
			} else {
				require.Error(t, err, desc)
			}
			require.Equal(t, c.done, done, desc)
		}
	}
}

func TestMakeCommitmentCounterConcurrent(t *testing.T) {
	const nrFaulty = 2
	const nrReplicas = 2*nrFaulty + 1
	const nrPrepares = 100
	const nrViews = 10

	counter := makeCommitmentCounter(nrFaulty)

	wg := new(sync.WaitGroup)
	for v := 0; v < nrViews; v++ {
		v := v
		firstCV := 1 + v*rand.Intn(nrPrepares)
		primary := v % nrReplicas

		for id := 0; id < nrReplicas; id++ {
			id := id

			wg.Add(1)
			go func() {
				defer wg.Done()

				for i := 0; i <= nrPrepares; i++ {
					// We can't check how many times the
					// counter was invoked before
					// signaling done and still invoke it
					// concurrently. So we only check for
					// data races here.
					cv := firstCV + i
					_, err := counter(uint32(id), makePrepare(primary, v, cv))
					assert.NoError(t, err, "Replica %d, Prepare %d", id, cv)
				}
			}()
		}
	}
	wg.Wait()
}

func makePrepare(p, v, cv int) *messages.Prepare {
	prepareUI := &usig.UI{
		Counter: uint64(cv),
	}
	prepareUIBytes, _ := prepareUI.MarshalBinary()

	return &messages.Prepare{
		Msg: &messages.Prepare_M{
			ReplicaId: uint32(p),
			View:      uint64(v),
		},
		ReplicaUi: prepareUIBytes,
	}
}
