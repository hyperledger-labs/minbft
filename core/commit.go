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

	usigui "github.com/hyperledger-labs/minbft/core/internal/usig-ui"
	"github.com/hyperledger-labs/minbft/messages"
)

// commitValidator validates a Commit message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type commitValidator func(commit messages.Commit) error

// commitApplier applies Commit message to current replica state.
//
// The supplied message is applied to the current replica state by
// changing the state accordingly and producing any required side
// effects. The supplied message is assumed to be authentic and
// internally consistent. Parameter active indicates if the message
// refers to the active view. It is safe to invoke concurrently.
type commitApplier func(commit messages.Commit, active bool) error

// commitmentCollector collects replica commitment.
//
// Each supplied message representing a replica commitment should be
// already validated and passed exactly once following the sequence of
// the assigned UI. If the threshold of matching commitments from
// distinct replicas has been reached, it triggers further actions to
// execute the committed proposal. It is safe to invoke concurrently.
type commitmentCollector func(msg messages.CertifiedMessage) error

// commitmentAcceptor checks replica commitment for consistency.
//
// It checks if the commitment represented by the supplied parameters
// is consistent with previously accepted commitments. The parameter
// replicaID specified the committed replica identifier. The parameter
// newView indicates if the commitment refers to the first primary's
// proposal in a new view. The parameters view, primaryCV, replicaCV
// specify the view number, the primary's and the replica's UI counter
// values respectively.
type commitmentAcceptor func(replicaID uint32, newView bool, view, primaryCV, replicaCV uint64) error

// commitmentCounter counts replica commitments.
//
// The supplied parameters represent primary's proposal referred by
// the commitment. The return value indicates if the acceptance
// threshold has been reached for the primary's proposal. The
// parameters view and primaryCV specify the view number, and the
// primary's UI counter value respectively.
type commitmentCounter func(view, primaryCV uint64) (done bool)

// makeCommitValidator constructs an instance of commitValidator using
// the supplied abstractions.
func makeCommitValidator(verifyUI usigui.UIVerifier, validatePrepare prepareValidator) commitValidator {
	return func(commit messages.Commit) error {
		prepare := commit.Prepare()

		if commit.ReplicaID() == prepare.ReplicaID() {
			return fmt.Errorf("Commit from primary")
		}

		if err := validatePrepare(prepare); err != nil {
			return fmt.Errorf("Invalid Prepare: %s", err)
		}

		if err := verifyUI(commit); err != nil {
			return fmt.Errorf("UI is not valid: %s", err)
		}

		return nil
	}
}

// makeCommitApplier constructs an instance of commitApplier using the
// supplied abstractions.
func makeCommitApplier(collectCommitment commitmentCollector) commitApplier {
	return func(commit messages.Commit, active bool) error {
		if err := collectCommitment(commit); err != nil {
			return fmt.Errorf("Commit cannot be taken into account: %s", err)
		}

		return nil
	}
}

// makeCommitmentCollector constructs an instance of
// commitmentCollector using the supplied abstractions.
func makeCommitmentCollector(acceptCommitment commitmentAcceptor, countCommitment commitmentCounter, executeRequest requestExecutor) commitmentCollector {
	var lock sync.Mutex

	return func(msg messages.CertifiedMessage) error {
		replicaID := msg.ReplicaID()

		var prepare messages.Prepare
		switch msg := msg.(type) {
		case messages.Prepare:
			prepare = msg
		case messages.Commit:
			prepare = msg.Prepare()
		default:
			return fmt.Errorf("Unexpected message type")
		}

		view := prepare.View()
		primaryCV := prepare.UI().Counter
		replicaCV := msg.UI().Counter

		lock.Lock()
		defer lock.Unlock()

		if err := acceptCommitment(replicaID, false, view, primaryCV, replicaCV); err != nil {
			return fmt.Errorf("Cannot accept commitment: %s", err)
		}

		if done := countCommitment(view, primaryCV); !done {
			return nil
		}

		executeRequest(prepare.Request())

		return nil
	}
}

func makeCommitmentAcceptor() commitmentAcceptor {
	var (
		replicaViews   = make(map[uint32]uint64)
		lastPrimaryCVs = make(map[uint32]uint64)
		lastReplicaCVs = make(map[uint32]uint64)
	)

	return func(replicaID uint32, newView bool, view, primaryCV, replicaCV uint64) error {
		if newView {
			if view <= replicaViews[replicaID] {
				return fmt.Errorf("Unexpected view number")
			}
			replicaViews[replicaID] = view
		} else {
			if view != replicaViews[replicaID] {
				return fmt.Errorf("Unexpected view number")
			}
			if primaryCV != lastPrimaryCVs[replicaID]+1 {
				return fmt.Errorf("Non-sequential primary UI")
			}
			if replicaCV != lastReplicaCVs[replicaID]+1 {
				return fmt.Errorf("Non-sequential replica UI")
			}
		}

		lastPrimaryCVs[replicaID] = primaryCV
		lastReplicaCVs[replicaID] = replicaCV

		return nil
	}
}

func makeCommitmentCounter(f uint32) commitmentCounter {
	var (
		lastView = uint64(0)
		highest  = make([]uint64, f)
	)

	return func(view, primaryCV uint64) (done bool) {
		if view < lastView {
			return false
		}
		if view > lastView {
			lastView = view
			highest = make([]uint64, f)
		}

		for i, cv := range highest {
			if primaryCV > cv {
				highest[i] = primaryCV
				return false
			}
		}

		return true
	}
}
