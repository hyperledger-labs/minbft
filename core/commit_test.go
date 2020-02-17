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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testifymock "github.com/stretchr/testify/mock"
	yaml "gopkg.in/yaml.v2"

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

	verifyUI := func(msg messages.CertifiedMessage) error {
		args := mock.MethodCalled("uiVerifier", msg)
		return args.Error(0)
	}
	validatePrepare := func(prepare messages.Prepare) error {
		args := mock.MethodCalled("prepareValidator", prepare)
		return args.Error(0)
	}
	validate := makeCommitValidator(verifyUI, validatePrepare)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(primary, view, request)

	commit := messageImpl.NewCommit(primary, prepare)
	err := validate(commit)
	assert.Error(t, err, "Commit from primary")

	commit = messageImpl.NewCommit(backup, prepare)

	mock.On("prepareValidator", prepare).Return(fmt.Errorf("UI not valid")).Once()
	err = validate(commit)
	assert.Error(t, err, "Invalid Prepare")

	mock.On("prepareValidator", prepare).Return(nil).Once()
	mock.On("uiVerifier", commit).Return(fmt.Errorf("UI not valid")).Once()
	err = validate(commit)
	assert.Error(t, err, "Invalid UI")

	mock.On("prepareValidator", prepare).Return(nil).Once()
	mock.On("uiVerifier", commit).Return(nil).Once()
	err = validate(commit)
	assert.NoError(t, err)
}

func TestMakeCommitApplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	collectCommitment := func(msg messages.CertifiedMessage) error {
		args := mock.MethodCalled("commitmentCollector", msg)
		return args.Error(0)
	}
	apply := makeCommitApplier(collectCommitment)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	id := randOtherReplicaID(primary, n)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(primary, view, request)
	commit := messageImpl.NewCommit(id, prepare)

	mock.On("commitmentCollector", commit).Return(fmt.Errorf("error")).Once()
	err := apply(commit, true)
	assert.Error(t, err, "Failed to collect commitment")

	mock.On("commitmentCollector", commit).Return(nil).Once()
	err = apply(commit, false)
	assert.NoError(t, err)

	mock.On("commitmentCollector", commit).Return(nil).Once()
	err = apply(commit, true)
	assert.NoError(t, err)
}

func TestMakeCommitmentCollector(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	acceptCommitment := func(id uint32, nv bool, view, primaryCV, replicaCV uint64) error {
		args := mock.MethodCalled("commitmentAcceptor", id, nv, view, primaryCV, replicaCV)
		return args.Error(0)
	}
	countCommitment := func(view, primaryCV uint64) (done bool) {
		args := mock.MethodCalled("commitmentCounter", view, primaryCV)
		return args.Bool(0)
	}
	executeRequest := func(request messages.Request) {
		mock.MethodCalled("requestExecutor", request)
	}
	collect := makeCommitmentCollector(acceptCommitment, countCommitment, executeRequest)

	n := randN()
	view := randView()
	primaryCV := rand.Uint64()
	backupCV := rand.Uint64()
	primary := primaryID(n, view)
	id := randOtherReplicaID(primary, n)
	clientID := rand.Uint32()

	request := messageImpl.NewRequest(clientID, rand.Uint64(), nil)
	prepare := messageImpl.NewPrepare(primary, view, request)
	prepare.SetUI(&usig.UI{Counter: primaryCV})
	commit := messageImpl.NewCommit(id, prepare)
	commit.SetUI(&usig.UI{Counter: backupCV})

	mock.On("commitmentAcceptor", primary, false, view, primaryCV, primaryCV).Return(fmt.Errorf("error")).Once()
	err := collect(prepare)
	assert.Error(t, err, "Failed to accept Prepare")

	mock.On("commitmentAcceptor", id, false, view, primaryCV, backupCV).Return(fmt.Errorf("error")).Once()
	err = collect(commit)
	assert.Error(t, err, "Failed to accept Commit")

	mock.On("commitmentAcceptor", primary, false, view, primaryCV, primaryCV).Return(nil).Once()
	mock.On("commitmentCounter", view, primaryCV).Return(false).Once()
	err = collect(prepare)
	assert.NoError(t, err)

	mock.On("commitmentAcceptor", id, false, view, primaryCV, backupCV).Return(nil).Once()
	mock.On("commitmentCounter", view, primaryCV).Return(false).Once()
	err = collect(commit)
	assert.NoError(t, err)

	mock.On("commitmentAcceptor", primary, false, view, primaryCV, primaryCV).Return(nil).Once()
	mock.On("commitmentCounter", view, primaryCV).Return(true).Once()
	mock.On("requestExecutor", request).Once()
	err = collect(prepare)
	assert.NoError(t, err)

	mock.On("commitmentAcceptor", id, false, view, primaryCV, backupCV).Return(nil).Once()
	mock.On("commitmentCounter", view, primaryCV).Return(true).Once()
	mock.On("requestExecutor", request).Once()
	err = collect(commit)
	assert.NoError(t, err)
}

func TestMakeCommitmentCollectorConcurrent(t *testing.T) {
	const nrFaulty = 1
	const nrReplicas = 100
	const nrPrepares = 100

	var executedReqs []messages.Request
	var lastSeq uint64

	commitCertSize := uint32(nrFaulty + 1)
	acceptCommitment := makeCommitmentAcceptor()
	countCommitment := makeCommitmentCounter(commitCertSize)
	executeRequest := func(req messages.Request) {
		// Real requestExecutor ensures exactly-once
		// semantics; just imitate this behavior here
		seq := req.Sequence()
		if seq <= lastSeq {
			return
		}
		lastSeq = seq

		time.Sleep(time.Millisecond)
		executedReqs = append(executedReqs, req)
	}
	collect := makeCommitmentCollector(acceptCommitment, countCommitment, executeRequest)

	wg := new(sync.WaitGroup)
	for id := 0; id < nrReplicas; id++ {
		id := uint32(id)

		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i <= nrPrepares; i++ {
				cv := uint64(i + 1)
				seq := cv

				request := messageImpl.NewRequest(0, seq, nil)

				prepare := messageImpl.NewPrepare(0, 0, request)
				prepare.SetUI(&usig.UI{Counter: cv})

				var msg messages.CertifiedMessage
				if id == prepare.ReplicaID() {
					msg = prepare
				} else {
					msg = messageImpl.NewCommit(id, prepare)
					msg.SetUI(&usig.UI{Counter: cv})
				}

				err := collect(msg)
				assert.NoError(t, err, "Replica %d, Prepare %d", id, cv)
			}
		}()
	}

	wg.Wait()

	for i, req := range executedReqs {
		assert.Equal(t, uint64(i+1), req.Sequence())
	}
}

func TestMakeCommitmentAcceptor(t *testing.T) {
	var cases []struct {
		ID        uint32
		NewView   bool
		View      uint64
		PrimaryCV uint64
		ReplicaCV uint64
		Ok        bool
	}
	casesYAML := []byte(`
- {id: 0, newview: n, view: 0, primarycv: 1, replicacv:  1, ok: y}
- {id: 0, newview: n, view: 0, primarycv: 2, replicacv:  2, ok: y}
- {id: 0, newview: n, view: 0, primarycv: 2, replicacv:  3, ok: n}
- {id: 1, newview: n, view: 0, primarycv: 1, replicacv:  1, ok: y}
- {id: 1, newview: n, view: 0, primarycv: 3, replicacv:  2, ok: n}
- {id: 2, newview: n, view: 0, primarycv: 1, replicacv:  1, ok: y}
- {id: 2, newview: n, view: 0, primarycv: 2, replicacv:  3, ok: n}
- {id: 3, newview: n, view: 0, primarycv: 1, replicacv:  1, ok: y}
- {id: 3, newview: n, view: 0, primarycv: 2, replicacv:  2, ok: y}
- {id: 3, newview: y, view: 1, primarycv: 2, replicacv:  4, ok: y}
- {id: 3, newview: n, view: 1, primarycv: 3, replicacv:  5, ok: y}
- {id: 3, newview: y, view: 2, primarycv: 2, replicacv:  7, ok: y}
- {id: 3, newview: y, view: 4, primarycv: 4, replicacv:  9, ok: y}
- {id: 3, newview: n, view: 5, primarycv: 5, replicacv: 11, ok: n}
- {id: 4, newview: y, view: 1, primarycv: 2, replicacv:  2, ok: y}
- {id: 4, newview: n, view: 0, primarycv: 1, replicacv:  3, ok: n}
- {id: 5, newview: y, view: 0, primarycv: 1, replicacv:  2, ok: n}
- {id: 6, newview: y, view: 1, primarycv: 2, replicacv:  2, ok: y}
- {id: 6, newview: y, view: 0, primarycv: 1, replicacv:  4, ok: n}
`)
	if err := yaml.UnmarshalStrict(casesYAML, &cases); err != nil {
		t.Fatal(err)
	}

	accept := makeCommitmentAcceptor()
	for i, c := range cases {
		desc := fmt.Sprintf("Case #%d", i)
		err := accept(c.ID, c.NewView, c.View, c.PrimaryCV, c.ReplicaCV)
		if c.Ok {
			require.NoError(t, err, desc)
		} else {
			require.Error(t, err, desc)
		}
	}
}

func TestMakeCommitmentCounter(t *testing.T) {
	const f = 1

	var cases []struct {
		View uint64
		CV   uint64
		Done bool
	}
	casesYAML := []byte(`
- {view: 0, cv: 1, done: n}
- {view: 0, cv: 1, done: y}
- {view: 0, cv: 1, done: y}
- {view: 0, cv: 2, done: n}
- {view: 0, cv: 3, done: n}
- {view: 1, cv: 2, done: n}
- {view: 1, cv: 3, done: n}
- {view: 0, cv: 2, done: n}
- {view: 0, cv: 2, done: n}
- {view: 0, cv: 3, done: n}
- {view: 0, cv: 3, done: n}
- {view: 1, cv: 2, done: y}
- {view: 1, cv: 3, done: y}
- {view: 1, cv: 2, done: y}
- {view: 1, cv: 3, done: y}
`)
	if err := yaml.UnmarshalStrict(casesYAML, &cases); err != nil {
		t.Fatal(err)
	}

	commitCertSize := uint32(f + 1)
	count := makeCommitmentCounter(commitCertSize)
	for i, c := range cases {
		desc := fmt.Sprintf("Case #%d", i)
		done := count(c.View, c.CV)
		require.Equal(t, c.Done, done, desc)
	}
}
