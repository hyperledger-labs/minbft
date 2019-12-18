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
	"github.com/hyperledger-labs/minbft/core/internal/peerstate"
	"github.com/hyperledger-labs/minbft/usig"

	messages "github.com/hyperledger-labs/minbft/messages/protobuf"

	mock_api "github.com/hyperledger-labs/minbft/api/mocks"
	mock_peerstate "github.com/hyperledger-labs/minbft/core/internal/peerstate/mocks"
	mock_messages "github.com/hyperledger-labs/minbft/messages/protobuf/mocks"
)

func TestMakeUIVerifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authen := mock_api.NewMockAuthenticator(ctrl)
	makeMsg := func(cv uint64) (messages.MessageWithUI, *usig.UI) {
		return makeMockUIMsg(ctrl, rand.Uint32(), cv)
	}

	verifyUI := makeUIVerifier(authen)

	cv := rand.Uint64()

	// Correct UI
	msg, expectedUI := makeMsg(cv)
	authen.EXPECT().VerifyMessageAuthenTag(
		api.USIGAuthen, msg.ReplicaID(), msg.Payload(), msg.UIBytes(),
	).Return(nil)
	actualUI, err := verifyUI(msg)
	assert.NoError(t, err)
	assert.Equal(t, expectedUI, actualUI)

	// Failed USIG certificate verification
	msg, _ = makeMsg(cv)
	authen.EXPECT().VerifyMessageAuthenTag(
		api.USIGAuthen, msg.ReplicaID(), msg.Payload(), msg.UIBytes(),
	).Return(fmt.Errorf("USIG certificate invalid"))
	actualUI, err = verifyUI(msg)
	assert.Error(t, err)
	assert.Nil(t, actualUI)

	// Invalid (zero) counter value
	msg, _ = makeMsg(uint64(0))
	actualUI, err = verifyUI(msg)
	assert.Error(t, err)
	assert.Nil(t, actualUI)
}

func TestMakeUICapturer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	replicaID := rand.Uint32()
	providePeerState, peerState := setupPeerStateProviderMock(ctrl, mock, replicaID)

	captureUI := makeUICapturer(providePeerState)

	msg, ui := makeMockUIMsg(ctrl, replicaID, rand.Uint64())

	peerState.EXPECT().CaptureUI(ui).Return(false, nil)
	new, _ := captureUI(msg)
	assert.False(t, new)

	peerState.EXPECT().CaptureUI(ui).Return(true, func() {
		mock.MethodCalled("uiReleaser", ui)
	})
	new, releaseUI := captureUI(msg)
	assert.True(t, new)
	mock.On("uiReleaser", ui).Once()
	releaseUI()
}

func setupPeerStateProviderMock(ctrl *gomock.Controller, mock *testifymock.Mock, replicaID uint32) (peerstate.Provider, *mock_peerstate.MockState) {
	peerState := mock_peerstate.NewMockState(ctrl)

	providePeerState := func(replicaID uint32) peerstate.State {
		args := mock.MethodCalled("peerStateProvider", replicaID)
		return args.Get(0).(peerstate.State)
	}

	mock.On("peerStateProvider", replicaID).Return(peerState)

	return providePeerState, peerState
}

func makeMockUIMsg(ctrl *gomock.Controller, replicaID uint32, cv uint64) (messages.MessageWithUI, *usig.UI) {
	msg := mock_messages.NewMockMessageWithUI(ctrl)
	msg.EXPECT().ReplicaID().Return(replicaID).AnyTimes()

	payload := make([]byte, 1)
	rand.Read(payload)
	msg.EXPECT().Payload().Return(payload).AnyTimes()

	cert := make([]byte, 1)
	rand.Read(cert)
	ui := &usig.UI{Counter: cv, Cert: cert}
	uiBytes, _ := ui.MarshalBinary()
	msg.EXPECT().UIBytes().Return(uiBytes).AnyTimes()

	return msg, ui
}
