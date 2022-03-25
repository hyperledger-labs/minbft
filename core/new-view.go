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

// newViewApplier applies NewView message to current replica state.
//
// The supplied message is applied to the current replica state by
// changing the state accordingly and producing any required messages
// or side effects. The supplied message is assumed to be authentic
// and internally consistent. It is safe to invoke concurrently.
type newViewApplier func(nv messages.NewView) error

// newViewAcceptor completes transitioning into the new active view.
//
// It brings the replica state according to the supplied NewView
// messages and completes transitioning into the new active view.
type newViewAcceptor func(nv messages.NewView)

// preparedRequestExtractor extracts a sequence of prepared requests
// from a new-view certificate.
//
// Given a new-view certificate, it returns a sequence of requests
// that appear prepared in the new-view certificate. The returned
// slice thus represents the sequence of possibly committed (executed)
// requests evidenced by the supplied new-view certificate.
type preparedRequestExtractor func(nvCert messages.NewViewCert) (prepared []messages.Request)

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

func makeNewViewApplier(id uint32, extractPrepared preparedRequestExtractor, prepareReq requestPreparer, collectCommitment commitmentCollector, stopVCTimer viewChangeTimerStopper, applyPendingReqs pendingRequestApplier, handleGeneratedMessage generatedMessageHandler) newViewApplier {
	return func(nv messages.NewView) error {
		// Re-prepare requests propagated from the previous views
		for _, req := range extractPrepared(nv.NewViewCert()) {
			prepareReq(req)
		}

		if err := collectCommitment(nv); err != nil {
			return fmt.Errorf("NewView cannot be taken into account: %s", err)
		}

		stopVCTimer()
		applyPendingReqs(nv.NewView())

		if id == nv.ReplicaID() {
			return nil // do not generate Commit for own message
		}

		handleGeneratedMessage(messageImpl.NewCommit(id, nv))

		return nil
	}
}

func makeNewViewAcceptor(extractPrepared preparedRequestExtractor, executeRequest requestExecutor) newViewAcceptor {
	return func(nv messages.NewView) {
		for _, req := range extractPrepared(nv.NewViewCert()) {
			executeRequest(req)
		}
	}
}

func extractPreparedRequests(nvCert messages.NewViewCert) (prepared []messages.Request) {
	var vcs = make([]messages.ViewChange, 0, len(nvCert))
	var maxLog messages.MessageLog
	for _, vc := range nvCert {
		if l := vc.MessageLog(); len(l) > 0 {
			if vc, ok := l[0].(messages.ViewChange); ok {
				// Put the leading ViewChange message aside
				vcs = append(vcs, vc)
				l = l[1:]
			}
			if len(l) > len(maxLog) {
				maxLog = l
			}
		}
	}

	if len(maxLog) == 0 && len(vcs) > 0 {
		// No request prepared in this view;
		// descend into the previous view
		return extractPreparedRequests(vcs)
	}

	for _, m := range maxLog {
		if comm, ok := m.(messages.Commit); ok {
			m = comm.Proposal()
		}

		switch m := m.(type) {
		case messages.Prepare:
			prepared = append(prepared, m.Request())
		case messages.NewView:
			prepared = extractPreparedRequests(m.NewViewCert())
		default:
			panic("Unexpected message type")
		}
	}

	return prepared
}
