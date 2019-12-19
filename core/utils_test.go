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
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/messages"

	mock_api "github.com/hyperledger-labs/minbft/api/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/mocks"
)

func TestMakeReplicaMessageSigner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authen := mock_api.NewMockAuthenticator(ctrl)
	signer := makeReplicaMessageSigner(authen)

	payload := make([]byte, 1)
	signature := make([]byte, 1)
	rand.Read(payload)
	rand.Read(signature)
	msg := mock_messages.NewMockSignedMessage(ctrl)
	msg.EXPECT().SignedPayload().Return(payload).AnyTimes()

	err := fmt.Errorf("cannot sign")
	authen.EXPECT().GenerateMessageAuthenTag(api.ReplicaAuthen, payload).Return(nil, err)
	assert.Panics(t, func() { signer(msg) })

	authen.EXPECT().GenerateMessageAuthenTag(api.ReplicaAuthen, payload).Return(signature, nil)
	msg.EXPECT().SetSignature(signature)
	signer(msg)
}

func TestMakeMessageSignatureVerifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authen := mock_api.NewMockAuthenticator(ctrl)
	verifier := makeMessageSignatureVerifier(authen)

	clientID := rand.Uint32()
	payload := make([]byte, 1)
	signature := make([]byte, 1)
	rand.Read(payload)
	rand.Read(signature)

	signedMsg := mock_messages.NewMockSignedMessage(ctrl)
	signedMsg.EXPECT().SignedPayload().Return(payload).AnyTimes()
	signedMsg.EXPECT().Signature().Return(signature).AnyTimes()
	clientMsg := mock_messages.NewMockClientMessage(ctrl)
	clientMsg.EXPECT().ClientID().Return(clientID).AnyTimes()
	msg := struct {
		messages.ClientMessage
		messages.SignedMessage
	}{clientMsg, signedMsg}

	assert.Panics(t, func() { verifier(signedMsg) }, "Message with no signer ID")

	authen.EXPECT().VerifyMessageAuthenTag(api.ClientAuthen, clientID,
		payload, signature).Return(fmt.Errorf(""))
	err := verifier(msg)
	assert.Error(t, err)

	authen.EXPECT().VerifyMessageAuthenTag(api.ClientAuthen, clientID,
		payload, signature).Return(nil)
	err = verifier(msg)
	assert.NoError(t, err)
}
