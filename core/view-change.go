// Copyright (c) 2022 NEC Laboratories Europe GmbH.
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

// viewChangeValidator validates a ReqViewChangeMessage.
//
// It checks the supplied message for internal consistency. It does
// not use replica's current state and has no side-effect. It is safe
// to invoke concurrently.
type viewChangeValidator func(vc messages.ViewChange) error

// viewChangeCertValidator validates a view-change certificate.
//
// It checks the supplied view-change certificate for consistency and
// completeness. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type viewChangeCertValidator func(newView uint64, cert messages.ViewChangeCert) error

// messageLogValidator validates a replica's message log.
//
// It checks the supplied message log for completeness and
// authenticity, given the log's replica ID, as well as the expected
// view and next UI's counter values. It does not use replica's
// current state and has no side-effect. It is safe to invoke
// concurrently.
type messageLogValidator func(replicaID uint32, view uint64, log messages.MessageLog, nextCV uint64) error

func makeViewChangeValidator(validateMessageLog messageLogValidator, validateVCCert viewChangeCertValidator) viewChangeValidator {
	return func(vc messages.ViewChange) error {
		newView := vc.NewView()

		if newView < 1 {
			return fmt.Errorf("invalid new view number")
		}

		if err := validateVCCert(newView, vc.ViewChangeCert()); err != nil {
			return fmt.Errorf("invalid view-change certificate: %s", err)
		}

		if err := validateMessageLog(vc.ReplicaID(), newView-1, vc.MessageLog(), vc.UI().Counter); err != nil {
			return fmt.Errorf("invalid message log: %s", err)
		}

		return nil
	}
}

func makeViewChangeCertValidator(viewChangeCertSize uint32) viewChangeCertValidator {
	return func(newView uint64, cert messages.ViewChangeCert) error {
		if len(cert) < int(viewChangeCertSize) {
			return fmt.Errorf("insufficient quorum")
		}

		q := make(map[uint32]bool, len(cert))
		for _, rvc := range cert {
			if rvc.NewView() != newView {
				return fmt.Errorf("new view number mismatch in %s", messages.Stringify(rvc))
			}

			replicaID := rvc.ReplicaID()
			if q[replicaID] {
				return fmt.Errorf("duplicate %s", messages.Stringify(rvc))
			}
			q[replicaID] = true
		}

		return nil
	}
}

func validateMessageLog(replicaID uint32, view uint64, log messages.MessageLog, nextCV uint64) error {
	if len(log) == 0 && (view > 0 || nextCV != 1) {
		return fmt.Errorf("incomplete log")
	}

	for i := len(log) - 1; i >= 0; i-- {
		m := log[i]

		if m.ReplicaID() != replicaID {
			return fmt.Errorf("foreign message %s", messages.Stringify(m))
		}

		if m.UI().Counter+1 != nextCV {
			return fmt.Errorf("unexpected UI after %s", messages.Stringify(m))
		}
		nextCV--

		switch m := m.(type) {
		case messages.ViewChange, messages.NewView:
			var msgView uint64
			switch m := m.(type) {
			case messages.ViewChange:
				msgView = m.NewView()
			case messages.NewView:
				msgView = m.NewView()
			}
			if msgView != view {
				return fmt.Errorf("log for unexpected view %d", msgView)
			}

			if i != 0 {
				return fmt.Errorf("unexpected message before %s", messages.Stringify(m))
			}
		default:
			if i == 0 && (view > 0 || nextCV != 1) {
				return fmt.Errorf("missing message before %s", messages.Stringify(m))
			}
		}
	}

	return nil
}
