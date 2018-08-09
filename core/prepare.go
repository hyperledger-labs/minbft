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

// makePrepareHandler constructs and instance of prepareHandler using
// id as the current replica ID, n as the total number of nodes, and
// the supplied abstract interfaces. The returned handler will send
// generated Commit messages to out channel for UI assignment and
// delivery to peer replicas.
func makePrepareHandler(id, n uint32, view viewProvider, acceptUI uiAcceptor, handleRequest requestHandler, collectCommit commitCollector, out chan<- messages.MessageWithUI) prepareHandler {
	return func(prepare *messages.Prepare) (new bool, err error) {
		logger.Debugf("Replica %d handling Prepare from replica %d: view=%d client=%d seq=%d",
			id, prepare.Msg.ReplicaId, prepare.Msg.View,
			prepare.Msg.Request.Msg.ClientId, prepare.Msg.Request.Msg.Seq)
		currentView := view()

		if new, err = acceptUI(prepare); err != nil {
			return false, fmt.Errorf("Prepare UI cannot be accepted: %s", err)
		}

		if prepare.Msg.View != currentView {
			return false, fmt.Errorf("Prepare is for view %d, current view is %d",
				prepare.Msg.View, currentView)
		} else if !isPrimary(currentView, prepare.ReplicaID(), n) {
			return false, fmt.Errorf("Prepare from backup %d in view %d",
				prepare.Msg.ReplicaId, currentView)
		}

		if _, _, err = handleRequest(prepare.Msg.Request, true); err != nil {
			return false, fmt.Errorf("Failed to process request: %s", err)
		}
		if !new || isPrimary(currentView, id, n) {
			return false, nil
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
		logger.Debugf("Replica %d generated Commit: view=%d primary=%d client=%d seq=%d",
			id, commit.Msg.View, commit.Msg.PrimaryId,
			commit.Msg.Request.Msg.ClientId, commit.Msg.Request.Msg.Seq)
		out <- commit

		return new, nil
	}
}
