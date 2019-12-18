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

	"github.com/hyperledger-labs/minbft/messages"
)

// prepareValidator validates a Prepare message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type prepareValidator func(prepare messages.Prepare) error

// prepareApplier applies Prepare message to current replica state.
//
// The supplied message is applied to the current replica state by
// changing the state accordingly and producing any required messages
// or side effects. The supplied message is assumed to be authentic
// and internally consistent. It is safe to invoke concurrently.
type prepareApplier func(prepare messages.Prepare) error

// makePrepareValidator constructs an instance of prepareValidator
// using n as the total number of nodes, and the supplied abstract
// interfaces.
func makePrepareValidator(n uint32, verifyUI uiVerifier, validateRequest requestValidator) prepareValidator {
	return func(prepare messages.Prepare) error {
		replicaID := prepare.ReplicaID()
		view := prepare.View()

		if !isPrimary(view, replicaID, n) {
			return fmt.Errorf("Prepare from backup %d for view %d", replicaID, view)
		}

		if err := validateRequest(prepare.Request()); err != nil {
			return fmt.Errorf("Request invalid: %s", err)
		}

		if _, err := verifyUI(prepare); err != nil {
			return fmt.Errorf("UI not valid: %s", err)
		}

		return nil
	}
}

// makePrepareApplier constructs an instance of prepareApplier using
// id as the current replica ID, and the supplied abstract interfaces.
func makePrepareApplier(id uint32, prepareSeq requestSeqPreparer, collectCommitment commitmentCollector, handleGeneratedUIMessage generatedUIMessageHandler, stopPrepTimer prepareTimerStopper) prepareApplier {
	return func(prepare messages.Prepare) error {
		request := prepare.Request()

		if new := prepareSeq(request); !new {
			return fmt.Errorf("Request already prepared")
		}

		primaryID := prepare.ReplicaID()

		if err := collectCommitment(primaryID, prepare); err != nil {
			return fmt.Errorf("Prepare cannot be taken into account: %s", err)
		}

		if id == primaryID {
			return nil // primary does not generate Commit
		}

		stopPrepTimer(request)

		commit := messageImpl.NewCommit(id, prepare)
		handleGeneratedUIMessage(commit)

		return nil
	}
}
