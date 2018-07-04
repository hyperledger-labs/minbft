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

	"github.com/nec-blockchain/minbft/api"

	mock_api "github.com/nec-blockchain/minbft/api/mocks"
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
	msg := &fakeMsgWithSignature{payload: payload}

	err := fmt.Errorf("cannot sign")
	authen.EXPECT().GenerateMessageAuthenTag(api.ReplicaAuthen, payload).Return(nil, err)
	assert.Panics(t, func() { signer(msg) })

	authen.EXPECT().GenerateMessageAuthenTag(api.ReplicaAuthen, payload).Return(signature, nil)
	assert.NotPanics(t, func() { signer(msg) })
	assert.Equal(t, signature, msg.signature)
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
	msgWithSig := &fakeMsgWithSignature{
		payload:   payload,
		signature: signature,
	}
	clientMsg := &fakeClientMsg{
		id:                   clientID,
		fakeMsgWithSignature: msgWithSig,
	}

	assert.Panics(t, func() {
		verifier(msgWithSig)
	}, "Message with no signer ID")

	err := fmt.Errorf("invalid signature")
	authen.EXPECT().VerifyMessageAuthenTag(api.ClientAuthen, clientID,
		payload, signature).Return(err)
	err = verifier(clientMsg)
	assert.Error(t, err)

	authen.EXPECT().VerifyMessageAuthenTag(api.ClientAuthen, clientID,
		payload, signature).Return(nil)
	err = verifier(clientMsg)
	assert.NoError(t, err)
}

type fakeMsgWithSignature struct {
	payload   []byte
	signature []byte
}

func (m *fakeMsgWithSignature) Payload() []byte {
	return m.payload
}

func (m *fakeMsgWithSignature) SignatureBytes() []byte {
	return m.signature
}

func (m *fakeMsgWithSignature) AttachSignature(signature []byte) {
	m.signature = signature
}

type fakeClientMsg struct {
	*fakeMsgWithSignature
	id uint32
}

func (m *fakeClientMsg) ClientID() uint32 {
	return m.id
}
