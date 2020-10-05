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

package utils

import (
	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/messages"
)

// MessageSigner generates and attaches a normal replica signature to
// the supplied message modifying it directly.
type MessageSigner func(msg messages.SignedMessage)

// MessageSignatureVerifier verifies the signature attached to a
// message against the message data.
type MessageSignatureVerifier func(msg messages.SignedMessage) error

// AuthenBytesExtractor returns serialized representation of message's
// authenticated content.
type AuthenBytesExtractor func(msg messages.Message) []byte

// MakeMessageSigner constructs an instance of MessageSigner using the
// supplied external interface for message authentication.
func MakeMessageSigner(authen api.Authenticator, extractAuthenBytes AuthenBytesExtractor) MessageSigner {
	return func(msg messages.SignedMessage) {
		authenBytes := extractAuthenBytes(msg.(messages.Message))
		signature, err := authen.GenerateMessageAuthenTag(api.ReplicaAuthen, authenBytes)
		if err != nil {
			panic(err) // Supplied Authenticator must be able to sign
		}
		msg.SetSignature(signature)
	}
}

// MakeMessageSignatureVerifier constructs an instance of
// MessageSignatureVerifier using the supplied external interface for
// message authentication.
func MakeMessageSignatureVerifier(authen api.Authenticator, extractAuthenBytes AuthenBytesExtractor) MessageSignatureVerifier {
	return func(msg messages.SignedMessage) error {
		var role api.AuthenticationRole
		var id uint32

		switch msg := msg.(type) {
		case messages.ClientMessage:
			role = api.ClientAuthen
			id = msg.ClientID()
		case messages.ReplicaMessage:
			role = api.ReplicaAuthen
			id = msg.ReplicaID()
		default:
			panic("Message with no signer ID")
		}

		authenBytes := extractAuthenBytes(msg.(messages.Message))
		return authen.VerifyMessageAuthenTag(role, id, authenBytes, msg.Signature())
	}
}

// IsPrimary returns true if the replica is the primary node for the
// current view.
func IsPrimary(view uint64, id uint32, n uint32) bool {
	return uint64(id) == view%uint64(n)
}
