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

package utils

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/messages"

	mock_api "github.com/hyperledger-labs/minbft/api/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/mocks"
)

func TestMakeMessageSigner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	extractAuthenBytes := func(m messages.Message) []byte {
		args := mock.MethodCalled("AuthenBytesExtractor", m)
		return args.Get(0).([]byte)
	}
	authen := mock_api.NewMockAuthenticator(ctrl)
	signer := MakeMessageSigner(authen, extractAuthenBytes)

	authenBytes := RandBytes()
	signature := RandBytes()

	msg := struct {
		*mock_messages.MockMessage
		*mock_messages.MockSignedMessage
	}{
		mock_messages.NewMockMessage(ctrl),
		mock_messages.NewMockSignedMessage(ctrl),
	}

	mock.On("AuthenBytesExtractor", msg).Return(authenBytes)

	err := fmt.Errorf("cannot sign")
	authen.EXPECT().GenerateMessageAuthenTag(api.ReplicaAuthen, authenBytes).Return(nil, err)
	assert.Panics(t, func() { signer(msg) })

	authen.EXPECT().GenerateMessageAuthenTag(api.ReplicaAuthen, authenBytes).Return(signature, nil)
	msg.MockSignedMessage.EXPECT().SetSignature(signature)
	signer(msg)
}

func TestMakeMessageSignatureVerifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	extractAuthenBytes := func(m messages.Message) []byte {
		args := mock.MethodCalled("AuthenBytesExtractor", m)
		return args.Get(0).([]byte)
	}
	authen := mock_api.NewMockAuthenticator(ctrl)
	verify := MakeMessageSignatureVerifier(authen, extractAuthenBytes)

	clientID := rand.Uint32()
	replicaID := rand.Uint32()

	authenBytes := RandBytes()
	signature := RandBytes()

	signedMsg := mock_messages.NewMockSignedMessage(ctrl)
	signedMsg.EXPECT().Signature().Return(signature).AnyTimes()
	clientMsg := mock_messages.NewMockClientMessage(ctrl)
	clientMsg.EXPECT().ClientID().Return(clientID).AnyTimes()
	signedClientMsg := struct {
		messages.ClientMessage
		messages.SignedMessage
	}{clientMsg, signedMsg}

	replicaMsg := mock_messages.NewMockReplicaMessage(ctrl)
	replicaMsg.EXPECT().ReplicaID().Return(replicaID).AnyTimes()

	signedReplicaMsg := &struct {
		messages.ReplicaMessage
		messages.SignedMessage
	}{replicaMsg, signedMsg}

	mock.On("AuthenBytesExtractor", signedClientMsg).Return(authenBytes)
	mock.On("AuthenBytesExtractor", signedReplicaMsg).Return(authenBytes)

	assert.Panics(t, func() { verify(signedMsg) }, "Message with no signer ID")

	authen.EXPECT().VerifyMessageAuthenTag(api.ClientAuthen, clientID,
		authenBytes, signature).Return(fmt.Errorf(""))
	err := verify(signedClientMsg)
	assert.Error(t, err)

	authen.EXPECT().VerifyMessageAuthenTag(api.ClientAuthen, clientID,
		authenBytes, signature).Return(nil)
	err = verify(signedClientMsg)
	assert.NoError(t, err)

	authen.EXPECT().VerifyMessageAuthenTag(api.ReplicaAuthen, replicaID,
		authenBytes, signature).Return(fmt.Errorf(""))
	err = verify(signedReplicaMsg)
	assert.Error(t, err)

	authen.EXPECT().VerifyMessageAuthenTag(api.ReplicaAuthen, replicaID,
		authenBytes, signature).Return(nil)
	err = verify(signedReplicaMsg)
	assert.NoError(t, err)
}
