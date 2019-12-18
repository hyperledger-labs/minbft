// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Wenting Li <wenting.li@neclab.eu>
//          Sergey Fedorov <sergey.fedorov@neclab.eu>
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
	"sync"

	"github.com/hyperledger-labs/minbft/core/internal/requestlist"
	"github.com/hyperledger-labs/minbft/messages"
)

// commitValidator validates a Commit message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type commitValidator func(commit *messages.Commit) error

// commitApplier applies Commit message to current replica state.
//
// The supplied message is applied to the current replica state by
// changing the state accordingly and producing any required side
// effects. The supplied message is assumed to be authentic and
// internally consistent. It is safe to invoke concurrently.
type commitApplier func(commit *messages.Commit) error

// commitmentCollector collects commitment on prepared Request.
//
// The supplied Prepare message is assumed to be valid and should have
// a UI assigned. Once the threshold of matching commitments from
// distinct replicas has been reached, it triggers further required
// actions to complete the prepared Request. It is safe to invoke
// concurrently.
type commitmentCollector func(replicaID uint32, prepare *messages.Prepare) error

// commitmentCounter counts commitments on prepared Request.
//
// The supplied Prepare message is assumed to be valid and should have
// a UI assigned. The return value done indicates if enough
// commitments from different replicas are counted for the supplied
// Prepare, such that the threshold to execute the prepared operation
// has been reached. An error is returned if any inconsistency is
// detected.
type commitmentCounter func(replicaID uint32, prepare *messages.Prepare) (done bool, err error)

// makeCommitValidator constructs an instance of commitValidator using
// the supplied abstractions.
func makeCommitValidator(verifyUI uiVerifier, validatePrepare prepareValidator) commitValidator {
	return func(commit *messages.Commit) error {
		if commit.Msg.ReplicaId == commit.Msg.PrimaryId {
			return fmt.Errorf("Commit from primary")
		}

		if err := validatePrepare(commit.Prepare()); err != nil {
			return fmt.Errorf("Invalid Prepare: %s", err)
		}

		if _, err := verifyUI(commit); err != nil {
			return fmt.Errorf("UI is not valid: %s", err)
		}

		return nil
	}
}

// makeCommitApplier constructs an instance of commitApplier using the
// supplied abstractions.
func makeCommitApplier(collectCommitment commitmentCollector) commitApplier {
	return func(commit *messages.Commit) error {
		replicaID := commit.ReplicaID()
		prepare := commit.Prepare()

		if err := collectCommitment(replicaID, prepare); err != nil {
			return fmt.Errorf("Commit cannot be taken into account: %s", err)
		}

		return nil
	}
}

// makeCommitmentCollector constructs an instance of
// commitmentCollector using the supplied abstractions.
func makeCommitmentCollector(countCommitment commitmentCounter, retireSeq requestSeqRetirer, pendingReq requestlist.List, stopReqTimer requestTimerStopper, executeRequest requestExecutor) commitmentCollector {
	var lock sync.Mutex

	return func(replicaID uint32, prepare *messages.Prepare) error {
		lock.Lock()
		defer lock.Unlock()

		if done, err := countCommitment(replicaID, prepare); err != nil {
			return err
		} else if !done {
			return nil
		}

		request := prepare.Msg.Request

		if new := retireSeq(request); !new {
			return nil // request already accepted for execution
		}

		pendingReq.Remove(request.Msg.ClientId)
		stopReqTimer(request)
		executeRequest(request)

		return nil
	}
}

// makeCommitmentCounter constructs an instance of commitmentCounter
// given the number of tolerated faulty nodes.
func makeCommitmentCounter(f uint32) commitmentCounter {
	// Replica ID -> committed
	type replicasCommittedMap map[uint32]bool

	var (
		// Current view number
		view uint64

		// UI counter of the first Prepare in the view
		firstCV uint64 = 1

		// Primary UI counter of the last quorum
		lastDoneCV uint64

		// Primary UI counter -> replicasCommittedMap
		prepareStates = make(map[uint64]replicasCommittedMap)
	)

	return func(replicaID uint32, prepare *messages.Prepare) (done bool, err error) {
		primaryID := prepare.ReplicaID()
		prepareView := prepare.View()
		prepareUI, err := parseMessageUI(prepare)
		if err != nil {
			panic(err)
		}
		prepareCV := prepareUI.Counter

		if prepareView < view {
			return false, nil
		} else if prepareView > view {
			view = prepareView
			firstCV = prepareCV
			lastDoneCV = 0
			prepareStates = make(map[uint64]replicasCommittedMap)
		}

		if prepareCV <= lastDoneCV {
			return true, nil
		}

		replicasCommitted := prepareStates[prepareCV]
		if replicasCommitted == nil {
			replicasCommitted = replicasCommittedMap{
				primaryID: true,
			}
			prepareStates[prepareCV] = replicasCommitted
		}

		if replicaID != primaryID {
			for cv := prepareCV - 1; cv >= firstCV; cv-- {
				s := prepareStates[cv]
				if s[replicaID] {
					break
				} else if s != nil {
					return false, fmt.Errorf("Skipped commitment")
				}
			}

			if replicasCommitted[replicaID] {
				return false, fmt.Errorf("Duplicated commitment")
			}

			replicasCommitted[replicaID] = true
		}

		if len(replicasCommitted) <= int(f) {
			return false, nil
		}

		lastDoneCV = prepareCV
		delete(prepareStates, prepareCV)

		return true, nil
	}
}
