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
	"github.com/hyperledger-labs/minbft/usig"
)

// commitValidator validates a Commit message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type commitValidator func(commit *messages.Commit) error

// commitProcessor processes a valid Commit message.
//
// It fully processes the supplied message in the context of the
// current replica's state. The supplied message is assumed to be
// authentic and internally consistent. The return value new indicates
// if the message has not been processed by this replica before. It is
// safe to invoke concurrently.
type commitProcessor func(commit *messages.Commit) (new bool, err error)

// commitCollector accepts valid Commit messages and takes further
// actions when the threshold of the required number of matching
// Commit messages is reached. Supplied Commit messages do not have to
// have UI assigned.
type commitCollector func(commit *messages.Commit) error

// commitCounter counts matching Commit messages and signals as the
// threshold to execute the operation is reached. The signal is
// received only once and any subsequent Commit message for the
// corresponding Prepare is simply ignored. All Commit messages are
// assumed to be valid and do not have to have UI assigned. An error
// is returned if any inconsistency detected. It is safe to invoke
// concurrently.
type commitCounter func(commit *messages.Commit) (done bool, err error)

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

func makeCommitProcessor(id uint32, view viewProvider, captureUI uiCapturer, processPrepare prepareProcessor, collectCommit commitCollector) commitProcessor {
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

		if err := collectCommit(commit); err != nil {
			return false, fmt.Errorf("Commit cannot be taken into account: %s", err)
		}

		return new, nil
	}
}

// makeCommitCollector constructs an instance of commitCollector using
// the supplied abstractions.
func makeCommitCollector(countCommits commitCounter, retireSeq requestSeqRetirer, executeRequest requestExecutor) commitCollector {
	return func(commit *messages.Commit) error {
		if done, err := countCommits(commit); err != nil {
			return err
		} else if !done {
			return nil
		}

		request := commit.Request()

		if new := retireSeq(request); !new {
			// commitCounter should never let us reach here
			panic("Request already accepted for execution")
		}

		// TODO: This is probably the place to stop the
		// request timer.

		executeRequest(request)

		return nil
	}
}

// makeCommitCounter constructs an instance of commitCounter given the
// number of tolerated faulty nodes.
func makeCommitCounter(f uint32) commitCounter {
	// Replica ID -> committed
	type replicasCommittedMap map[uint32]bool

	var (
		lock       sync.Mutex
		lastDoneCV = uint64(0)
		// Prepare UI -> replicasCommittedMap
		prepareStates = make(map[uint64]replicasCommittedMap)
	)

	return func(commit *messages.Commit) (done bool, err error) {
		prepare := commit.Prepare()

		prepareUI := new(usig.UI)
		err = prepareUI.UnmarshalBinary(prepare.UIBytes())
		if err != nil {
			panic(err) // valid Commit must have full valid Prepare
		}
		prepareCV := prepareUI.Counter

		lock.Lock()
		defer lock.Unlock()

		if prepareCV <= lastDoneCV {
			return false, nil // ignore extra Commit
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

		if replicasCommitted[commit.ReplicaID()] {
			return false, fmt.Errorf("Duplicated Commit detected")
		}
		replicasCommitted[commit.ReplicaID()] = true

		if len(replicasCommitted) == int(f+1) {
			if prepareCV != lastDoneCV+1 {
				panic("Request must be accepted in sequence assigned by primary")
			}

			delete(prepareStates, prepareCV)
			lastDoneCV++

			return true, nil
		}

		return false, nil
	}
}
