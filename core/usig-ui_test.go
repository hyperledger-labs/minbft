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

	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"

	mock_api "github.com/hyperledger-labs/minbft/api/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/mocks"
)

func TestMakeUIVerifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	extractAuthenBytes := func(m messages.Message) []byte {
		args := mock.MethodCalled("authenBytesExtractor", m)
		return args.Get(0).([]byte)
	}
	authen := mock_api.NewMockAuthenticator(ctrl)
	verifyUI := makeUIVerifier(authen, extractAuthenBytes)

	id := rand.Uint32()
	ui := &usig.UI{Counter: rand.Uint64(), Cert: randBytes()}
	uiBytes := usig.MustMarshalUI(ui)
	authenBytes := randBytes()
	msg := mock_messages.NewMockCertifiedMessage(ctrl)
	msg.EXPECT().ReplicaID().Return(id).AnyTimes()

	// Correct UI
	msg.EXPECT().UI().Return(ui)
	mock.On("authenBytesExtractor", msg).Return(authenBytes).Once()
	authen.EXPECT().VerifyMessageAuthenTag(
		api.USIGAuthen, id, authenBytes, uiBytes,
	).Return(nil)
	err := verifyUI(msg)
	assert.NoError(t, err)

	// Failed USIG certificate verification
	msg.EXPECT().UI().Return(ui)
	mock.On("authenBytesExtractor", msg).Return(authenBytes).Once()
	authen.EXPECT().VerifyMessageAuthenTag(
		api.USIGAuthen, id, authenBytes, uiBytes,
	).Return(fmt.Errorf("USIG certificate invalid"))
	err = verifyUI(msg)
	assert.Error(t, err)

	// Invalid (zero) counter value
	msg.EXPECT().UI().Return(&usig.UI{Counter: 0, Cert: randBytes()})
	err = verifyUI(msg)
	assert.Error(t, err)
}
