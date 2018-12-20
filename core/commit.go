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

	"github.com/hyperledger-labs/minbft/messages"
)

// commitValidator validates a Commit message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type commitValidator func(commit *messages.Commit) error

// commitProcessor processes a valid Commit message.
//
// It fully processes the supplied message. The supplied message is
// assumed to be authentic and internally consistent. The return value
// new indicates if the message has not been processed by this replica
// before. It is safe to invoke concurrently.
type commitProcessor func(commit *messages.Commit) (new bool, err error)

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
// has been reached. Note that the supplied Prepare message implies a
// commitment from the primary. An error is returned if any
// inconsistency is detected. It is safe to invoke concurrently.
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

func makeCommitProcessor(id uint32, processPrepare prepareProcessor, captureUI uiCapturer, view viewProvider, applyCommit commitApplier) commitProcessor {
	return func(commit *messages.Commit) (new bool, err error) {
		replicaID := commit.ReplicaID()

		if replicaID == id {
			return false, nil
		}

		if _, err := processPrepare(commit.Prepare()); err != nil {
			return false, fmt.Errorf("Failed to process Prepare: %s", err)
		}

		new, releaseUI := captureUI(commit)
		if !new {
			return false, nil
		}
		defer releaseUI()

		if currentView := view(); commit.Msg.View != currentView {
			return false, fmt.Errorf("Commit is for view %d, current view is %d",
				commit.Msg.View, currentView)
		}

		if err := applyCommit(commit); err != nil {
			return false, fmt.Errorf("Failed to apply Commit: %s", err)
		}

		return true, nil
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
func makeCommitmentCollector(countCommitment commitmentCounter, retireSeq requestSeqRetirer, executeRequest requestExecutor) commitmentCollector {
	return func(replicaID uint32, prepare *messages.Prepare) error {
		if done, err := countCommitment(replicaID, prepare); err != nil {
			return err
		} else if !done {
			return nil
		}

		request := prepare.Msg.Request

		if new := retireSeq(request); !new {
			return nil // request already accepted for execution
		}

		// TODO: This is probably the place to stop the
		// request timer.

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
		lock sync.Mutex

		lastDoneCV uint64

		// Primary UI counter -> replicasCommittedMap
		prepareStates = make(map[uint64]replicasCommittedMap)
	)

	return func(replicaID uint32, prepare *messages.Prepare) (done bool, err error) {
		prepareUI, err := parseMessageUI(prepare)
		if err != nil {
			panic(err)
		}
		prepareCV := prepareUI.Counter

		lock.Lock()
		defer lock.Unlock()

		if prepareCV <= lastDoneCV {
			return true, nil
		}

		replicasCommitted := prepareStates[prepareCV]
		if replicasCommitted == nil {
			// Every Commit message must include an
			// equivalent of a corresponding Prepare
			// message, which in turn signifies a
			// commitment from the primary replica to the
			// assigned order of request execution.
			// Therefore the extracted Prepare message is
			// treated as a virtual Commit from the
			// primary, thus the primary replica is
			// initially marked in the map.
			replicasCommitted = map[uint32]bool{
				prepare.Msg.ReplicaId: true,
			}
			prepareStates[prepareCV] = replicasCommitted
		}

		if replicasCommitted[replicaID] {
			return false, fmt.Errorf("Duplicated commitment")
		}
		replicasCommitted[replicaID] = true

		if len(replicasCommitted) == int(f+1) {
			delete(prepareStates, prepareCV)
			lastDoneCV++

			return true, nil
		}

		return false, nil
	}
}
