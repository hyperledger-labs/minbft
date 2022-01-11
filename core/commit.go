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
func makeCommitValidator(verifyUI uiVerifier) commitValidator {
	return func(commit messages.Commit) error {
		prop := commit.Proposal()

		if commit.ReplicaID() == prop.ReplicaID() {
			return fmt.Errorf("commit from primary")
		}

		switch prop.(type) {
		case messages.Prepare:
		default:
			panic("Unexpected proposal message type")
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
			return fmt.Errorf("commit cannot be taken into account: %s", err)
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

		var prop messages.CertifiedMessage
		switch msg := msg.(type) {
		case messages.Prepare:
			prop = msg
		case messages.Commit:
			prop = msg.Proposal()
		default:
			return fmt.Errorf("unexpected commitment message type")
		}

		var view uint64
		switch prop := prop.(type) {
		case messages.Prepare:
			view = prop.View()
		default:
			return fmt.Errorf("unexpected proposal message type")
		}

		primaryCV := prop.UI().Counter
		replicaCV := msg.UI().Counter

		lock.Lock()
		defer lock.Unlock()

		if err := acceptCommitment(replicaID, false, view, primaryCV, replicaCV); err != nil {
			return fmt.Errorf("cannot accept commitment: %s", err)
		}

		if done := countCommitment(view, primaryCV); !done {
			return nil
		}

		switch prop := prop.(type) {
		case messages.Prepare:
			executeRequest(prop.Request())
		default:
			panic("Unexpected proposal message type")
		}

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
				return fmt.Errorf("unexpected view number")
			}
			replicaViews[replicaID] = view
		} else {
			if view != replicaViews[replicaID] {
				return fmt.Errorf("unexpected view number")
			}
			if primaryCV != lastPrimaryCVs[replicaID]+1 {
				return fmt.Errorf("non-sequential primary UI")
			}
			if replicaCV != lastReplicaCVs[replicaID]+1 {
				return fmt.Errorf("non-sequential replica UI")
			}
		}

		lastPrimaryCVs[replicaID] = primaryCV
		lastReplicaCVs[replicaID] = replicaCV

		return nil
	}
}

// makeCommitmentCounter constructs an instance of commitmentCounter
// given the commit certificate size.
func makeCommitmentCounter(commitCertSize uint32) commitmentCounter {
	var (
		lastView = uint64(0)
		highest  = make([]uint64, commitCertSize-1)
	)

	return func(view, primaryCV uint64) (done bool) {
		if view < lastView {
			return false
		}
		if view > lastView {
			lastView = view
			highest = make([]uint64, len(highest))
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
