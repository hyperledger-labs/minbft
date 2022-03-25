// Copyright (c) 2021 NEC Laboratories Europe GmbH.
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

	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
	"github.com/hyperledger-labs/minbft/core/internal/viewstate"
	"github.com/hyperledger-labs/minbft/messages"
)

// reqViewChangeValidator validates a ReqViewChangeMessage.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type reqViewChangeValidator func(rvc messages.ReqViewChange) error

// reqViewChangeProcessor processes a valid ReqViewChange message.
//
// It continues processing of the supplied message. The return value
// new indicates if the message had any effect. It is safe to invoke
// concurrently.
type reqViewChangeProcessor func(rvc messages.ReqViewChange) (new bool, err error)

// reqViewChangeCollector collects view change requests.
//
// The supplied ReqViewChange message is assumed to be valid. Once the
// threshold of matching ReqViewChange messages from distinct replicas
// referring to the next view has been reached, it returns a
// view-change certificate comprised of those messages. The return
// value new indicates if the message had any effect.
type reqViewChangeCollector func(rvc messages.ReqViewChange) (new bool, _ messages.ViewChangeCert)

// viewChangeStarter attempts to start view change.
//
// It proceeds to trigger view change with the supplied expected new
// view number justified by the supplied view-change certificate
// unless the replica cannot transition to that view anymore.
type viewChangeStarter func(newView uint64, vcCert messages.ViewChangeCert) (ok bool, err error)

func makeReqViewChangeValidator(verifySignature messageSignatureVerifier) reqViewChangeValidator {
	return func(rvc messages.ReqViewChange) error {
		if rvc.NewView() < 1 {
			return fmt.Errorf("Invalid new view number")
		}

		if err := verifySignature(rvc); err != nil {
			return fmt.Errorf("Signature is not valid: %s", err)
		}

		return nil
	}
}

func makeReqViewChangeProcessor(collect reqViewChangeCollector, startViewChange viewChangeStarter) reqViewChangeProcessor {
	var lock sync.Mutex

	return func(rvc messages.ReqViewChange) (new bool, err error) {
		lock.Lock()
		defer lock.Unlock()

		new, vcCert := collect(rvc)
		if vcCert == nil {
			return new, nil
		}

		return startViewChange(rvc.NewView(), vcCert)
	}
}

func makeReqViewChangeCollector(viewChangeCertSize uint32) reqViewChangeCollector {
	var (
		view      uint64
		collected = make(messages.ViewChangeCert, 0, viewChangeCertSize)
		replicas  = make(map[uint32]bool, cap(collected))
	)

	return func(rvc messages.ReqViewChange) (new bool, vcCert messages.ViewChangeCert) {
		replicaID := rvc.ReplicaID()

		if rvc.NewView() != view+1 || replicas[replicaID] {
			return false, nil
		}

		collected = append(collected, rvc)
		replicas[replicaID] = true

		if len(collected) < int(viewChangeCertSize) {
			return true, nil
		}

		vcCert = collected
		collected = make(messages.ViewChangeCert, 0, cap(collected))
		replicas = make(map[uint32]bool, cap(collected))
		view++

		return true, vcCert
	}
}

func makeViewChangeStarter(id uint32, viewState viewstate.State, log messagelog.MessageLog, startVCTimer viewChangeTimerStarter, unprepareReqSeq requestSeqUnpreparer, handleGeneratedMessage generatedMessageHandler) viewChangeStarter {
	return func(newView uint64, vcCert messages.ViewChangeCert) (ok bool, err error) {
		ok, release := viewState.AdvanceExpectedView(newView)
		if !ok {
			return false, nil
		}
		defer release()

		var msgs messages.MessageLog
		for _, m := range log.Messages() {
			if m, ok := m.(messages.CertifiedMessage); ok {
				msgs = append(msgs, m)
			}
		}
		log.Reset(nil)

		startVCTimer(newView)
		unprepareReqSeq()

		handleGeneratedMessage(messageImpl.NewViewChange(id, newView, msgs, vcCert))

		return true, nil
	}
}
