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

// newViewValidator validates a NewViewMessage.
//
// It checks the supplied message for internal consistency. It does
// not use replica's current state and has no side-effect. It is safe
// to invoke concurrently.
type newViewValidator func(nv messages.NewView) error

// newViewCertValidator validates a new-view certificate.
//
// It checks the supplied new-view certificate for consistency and
// completeness. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type newViewCertValidator func(newPrimary uint32, newView uint64, cert messages.NewViewCert) error

func makeNewViewValidator(n uint32, validateNVCert newViewCertValidator) newViewValidator {
	return func(nv messages.NewView) error {
		replicaID := nv.ReplicaID()
		newView := nv.NewView()

		if newView < 1 {
			return fmt.Errorf("invalid new view number")
		}

		if !isPrimary(newView, replicaID, n) {
			return fmt.Errorf("NewView from backup replica")
		}

		if err := validateNVCert(replicaID, newView, nv.NewViewCert()); err != nil {
			return fmt.Errorf("invalid new-view certificate: %s", err)
		}

		return nil
	}
}

func makeNewViewCertValidator(viewChangeCertSize uint32) newViewCertValidator {
	return func(newPrimary uint32, newView uint64, cert messages.NewViewCert) error {
		if len(cert) < int(viewChangeCertSize) {
			return fmt.Errorf("insufficient quorum")
		}

		q := make(map[uint32]bool, len(cert))
		for _, vc := range cert {
			if vc.NewView() != newView {
				return fmt.Errorf("new view number mismatch in %s", messages.Stringify(vc))
			}

			replicaID := vc.ReplicaID()
			if q[replicaID] {
				return fmt.Errorf("duplicate %s", messages.Stringify(vc))
			}
			q[replicaID] = true
		}

		if !q[newPrimary] {
			return fmt.Errorf("missing ViewChange from new primary")
		}

		return nil
	}
}
