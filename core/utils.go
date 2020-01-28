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

	logging "github.com/op/go-logging"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/messages"
)

// messageSigner generates and attaches a normal replica signature to
// the supplied message modifying it directly.
type messageSigner func(msg messages.SignedMessage)

// messageSignatureVerifier verifies the signature attached to a
// message against the message data.
type messageSignatureVerifier func(msg messages.SignedMessage) error

// authenBytesExtractor returns serialized representation of message's
// authenticated content.
type authenBytesExtractor func(msg messages.Message) []byte

// makeMessageSigner constructs an instance of messageSigner using the
// supplied external interface for message authentication.
func makeMessageSigner(authen api.Authenticator, extractAuthenBytes authenBytesExtractor) messageSigner {
	return func(msg messages.SignedMessage) {
		authenBytes := extractAuthenBytes(msg.(messages.Message))
		signature, err := authen.GenerateMessageAuthenTag(api.ReplicaAuthen, authenBytes)
		if err != nil {
			panic(err) // Supplied Authenticator must be able to sign
		}
		msg.SetSignature(signature)
	}
}

// makeMessageSignatureVerifier constructs an instance of
// messageSignatureVerifier using the supplied external interface for
// message authentication.
func makeMessageSignatureVerifier(authen api.Authenticator, extractAuthenBytes authenBytesExtractor) messageSignatureVerifier {
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

// isPrimary returns true if the replica is the primary node for the
// current view.
func isPrimary(view uint64, id uint32, n uint32) bool {
	return uint64(id) == view%uint64(n)
}

func makeLogger(id uint32, opts options) *logging.Logger {
	logger := logging.MustGetLogger(module)
	logFormatString := fmt.Sprintf("%s Replica %d: %%{message}", defaultLogPrefix, id)
	stringFormatter := logging.MustStringFormatter(logFormatString)
	backend := logging.NewLogBackend(opts.logFile, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, stringFormatter)
	formattedLoggerBackend := logging.AddModuleLevel(backendFormatter)

	logger.SetBackend(formattedLoggerBackend)

	formattedLoggerBackend.SetLevel(opts.logLevel, module)

	return logger
}
