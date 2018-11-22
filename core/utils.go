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
	"os"

	"github.com/spf13/viper"
	logging "github.com/op/go-logging"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/messages"
)

// replicaMessageSigner generates and attaches a normal replica
// signature to the supplied message modifying it directly.
type replicaMessageSigner func(msg messages.MessageWithSignature)

// messageSignatureVerifier verifies the signature attached to a
// message against the message data.
type messageSignatureVerifier func(msg messages.MessageWithSignature) error

// viewProvider returns the current view this replica operates on.
type viewProvider func() (view uint64)

// makeReplicaMessageSigner constructs an instance of messageSigner
// using the supplied external interface for message authentication.
func makeReplicaMessageSigner(authen api.Authenticator) replicaMessageSigner {
	return func(msg messages.MessageWithSignature) {
		signature, err := authen.GenerateMessageAuthenTag(api.ReplicaAuthen, msg.Payload())
		if err != nil {
			panic(err) // Supplied Authenticator must be able to sing
		}
		msg.AttachSignature(signature)
	}
}

// makeMessageSignatureVerifier constructs an instance of
// messageSignatureVerifier using the supplied external interface for
// message authentication.
func makeMessageSignatureVerifier(authen api.Authenticator) messageSignatureVerifier {
	return func(msg messages.MessageWithSignature) error {
		var role api.AuthenticationRole
		var id uint32

		switch msg := msg.(type) {
		case messages.ClientMessage:
			role = api.ClientAuthen
			id = msg.ClientID()
		default:
			panic("Message with no signer ID")
		}

		payload := msg.Payload()
		signature := msg.SignatureBytes()

		return authen.VerifyMessageAuthenTag(role, id, payload, signature)
	}
}

// isPrimary returns true if the replica is the primary node for the
// current view.
func isPrimary(view uint64, id uint32, n uint32) bool {
	return uint64(id) == view%uint64(n)
}

func messageString(msg interface{}) string {
	var cv uint64
	if msg, ok := msg.(messages.MessageWithUI); ok {
		if ui, err := parseMessageUI(msg); err == nil {
			cv = ui.Counter
		}
	}

	switch msg := msg.(type) {
	case *messages.Request:
		m := msg.GetMsg()
		return fmt.Sprintf("REQUEST<client=%d seq=%d payload=%s>",
			m.GetClientId(), m.GetSeq(), m.GetPayload())
	case *messages.Reply:
		m := msg.GetMsg()
		return fmt.Sprintf("REPLY<replica=%d seq=%d result=%s>",
			m.GetReplicaId(), m.GetSeq(), m.GetResult())
	case *messages.Prepare:
		m := msg.GetMsg()
		req := m.GetRequest().GetMsg()
		return fmt.Sprintf("PREPARE<cv=%d replica=%d view=%d client=%d seq=%d>",
			cv, m.GetReplicaId(), m.GetView(),
			req.GetClientId(), req.GetSeq())
	case *messages.Commit:
		m := msg.GetMsg()
		req := m.GetRequest().GetMsg()
		return fmt.Sprintf("COMMIT<cv=%d replica=%d primary=%d view=%d client=%d seq=%d>",
			cv, m.GetReplicaId(), m.GetPrimaryId(), m.GetView(),
			req.GetClientId(), req.GetSeq())
	}
	return "(unknown message)"
}

func makeLogger(id uint32) (*logging.Logger, error) {
	logLevel := logging.DEBUG
	logBackend := os.Stdout
	var err error

	if viper.GetString("logging.level") != "" {
		logLevel, err = logging.LogLevel(viper.GetString("logging.level"))
		if err != nil {
			return nil, err
		}
	}

	if viper.GetString("logging.file") != "" {
		logBackend, err = os.OpenFile(viper.GetString("logging.file"),
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
	}

	logger := logging.MustGetLogger(module)
	logFormatString := fmt.Sprintf("%s Replica %d: %%{message}", defaultLogPrefix, id)
	stringFormatter := logging.MustStringFormatter(logFormatString)
	backend := logging.NewLogBackend(logBackend, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, stringFormatter)
	formattedLoggerBackend := logging.AddModuleLevel(backendFormatter)

	logger.SetBackend(formattedLoggerBackend)

	formattedLoggerBackend.SetLevel(logLevel, module)

	return logger, nil
}
