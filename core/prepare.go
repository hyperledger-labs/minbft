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

// prepareHandler fully handles a Prepare message. The Prepare message
// will be fully verified and processed. The return value new
// indicates that the valid message hasn't been processed before.
type prepareHandler func(prepare *messages.Prepare) (new bool, err error)

// prepareValidator validates a Prepare message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type prepareValidator func(prepare *messages.Prepare) error

// prepareProcessor processes a valid Prepare message.
//
// It fully processes the supplied message in the context of the
// current replica's state. The supplied message is assumed to be
// authentic and internally consistent. The return value new indicates
// if the message has not been processed by this replica before. It is
// safe to invoke concurrently.
type prepareProcessor func(prepare *messages.Prepare) (new bool, err error)

// makePrepareHandler constructs an instance of prepareHandler using
// the supplied abstract interfaces.
func makePrepareHandler(validate prepareValidator, process prepareProcessor) prepareHandler {
	return func(prepare *messages.Prepare) (new bool, err error) {
		if err = validate(prepare); err != nil {
			err = fmt.Errorf("Invalid message: %s", err)
			return false, err
		}

		return process(prepare)
	}
}

// makePrepareValidator constructs an instance of prepareValidator
// using n as the total number of nodes, and the supplied abstract
// interfaces.
func makePrepareValidator(n uint32, verifyUI uiVerifier, validateRequest requestValidator) prepareValidator {
	return func(prepare *messages.Prepare) error {
		replicaID := prepare.Msg.ReplicaId
		view := prepare.Msg.View

		if !isPrimary(view, replicaID, n) {
			return fmt.Errorf("Prepare from backup %d for view %d", replicaID, view)
		}

		if err := validateRequest(prepare.Msg.Request); err != nil {
			return fmt.Errorf("Request invalid: %s", err)
		}

		if _, err := verifyUI(prepare); err != nil {
			return fmt.Errorf("UI not valid: %s", err)
		}

		return nil
	}
}

// makePrepareProcessor constructs an instance of prepareProcessor
// using id as the current replica ID, and the supplied abstract
// interfaces.
func makePrepareProcessor(id uint32, view viewProvider, captureUI uiCapturer, prepareSeq requestSeqPreparer, processRequest requestProcessor, collectCommit commitCollector, handleGeneratedUIMessage generatedUIMessageHandler) prepareProcessor {
	return func(prepare *messages.Prepare) (new bool, err error) {
		replicaID := prepare.Msg.ReplicaId

		if replicaID == id {
			return false, nil
		}

		request := prepare.Msg.Request

		if _, err = processRequest(request); err != nil {
			return false, fmt.Errorf("Failed to process request: %s", err)
		}

		new, releaseUI := captureUI(prepare)
		if !new {
			return false, nil
		}
		defer releaseUI()

		currentView := view()

		if prepare.Msg.View != currentView {
			return false, fmt.Errorf("Prepare is for view %d, current view is %d",
				prepare.Msg.View, currentView)
		}

		if err = prepareSeq(request); err != nil {
			return false, fmt.Errorf("Failed to check request ID: %s", err)
		}

		commit := &messages.Commit{
			Msg: &messages.Commit_M{
				View:      currentView,
				ReplicaId: id,
				PrimaryId: prepare.ReplicaID(),
				Request:   prepare.Msg.Request,
				PrimaryUi: prepare.UIBytes(),
			},
		}
		if err := collectCommit(commit); err != nil {
			panic("Failed to collect own Commit")
		}

		handleGeneratedUIMessage(commit)

		return new, nil
	}
}
