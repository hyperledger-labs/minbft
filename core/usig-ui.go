// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
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

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
)

// uiVerifier verifies USIG certificate attached to a message.
//
// It verifies if the UI assigned to the message is correct and its
// USIG certificate is valid for the message. A UI with zero counter
// value is never valid.
type uiVerifier func(msg messages.CertifiedMessage) error

// uiAssigner assigns a unique identifier to a message.
//
// USIG UI is assigned and attached to the supplied message.
type uiAssigner func(msg messages.CertifiedMessage)

// makeUIVerifier constructs uiVerifier using the supplied external
// authenticator to verify USIG certificates.
func makeUIVerifier(authen api.Authenticator, extractAuthenBytes authenBytesExtractor) uiVerifier {
	return func(msg messages.CertifiedMessage) error {
		ui := msg.UI()
		if ui.Counter == uint64(0) {
			return fmt.Errorf("invalid (zero) counter value")
		}

		authenBytes := extractAuthenBytes(msg)
		uiBytes := usig.MustMarshalUI(ui)
		if err := authen.VerifyMessageAuthenTag(api.USIGAuthen, msg.ReplicaID(), authenBytes, uiBytes); err != nil {
			return fmt.Errorf("failed verifying USIG certificate: %s", err)
		}

		return nil
	}
}

// makeUIAssigner constructs uiAssigner using the supplied external
// authentication interface to generate USIG UIs.
func makeUIAssigner(authen api.Authenticator, extractAuthenBytes authenBytesExtractor) uiAssigner {
	return func(msg messages.CertifiedMessage) {
		authenBytes := extractAuthenBytes(msg)
		uiBytes, err := authen.GenerateMessageAuthenTag(api.USIGAuthen, authenBytes)
		if err != nil {
			panic(err)
		}
		ui := usig.MustUnmarshalUI(uiBytes)
		msg.SetUI(ui)
	}
}
