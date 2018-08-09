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

	"github.com/nec-blockchain/minbft/api"
	"github.com/nec-blockchain/minbft/messages"
	"github.com/nec-blockchain/minbft/usig"

	"github.com/nec-blockchain/minbft/core/internal/peerstate"

	mock_api "github.com/nec-blockchain/minbft/api/mocks"
	mock_peerstate "github.com/nec-blockchain/minbft/core/internal/peerstate/mocks"
	mock_messages "github.com/nec-blockchain/minbft/messages/mocks"
)

func TestMakeUIVerifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authen := mock_api.NewMockAuthenticator(ctrl)

	makeMsg := func(cv uint64) (messages.MessageWithUI, *usig.UI) {
		msg := mock_messages.NewMockMessageWithUI(ctrl)
		msg.EXPECT().ReplicaID().Return(rand.Uint32()).AnyTimes()

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

	ui := &usig.UI{Counter: rand.Uint64()}

	peerState.EXPECT().CaptureUI(ui).Return(false)
	new := captureUI(replicaID, ui)
	assert.False(t, new)

	peerState.EXPECT().CaptureUI(ui).Return(true)
	new = captureUI(replicaID, ui)
	assert.True(t, new)
}

func TestMakeUIReleaser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	replicaID := rand.Uint32()
	providePeerState, peerState := setupPeerStateProviderMock(ctrl, mock, replicaID)

	releaseUI := makeUIReleaser(providePeerState)

	ui := &usig.UI{Counter: rand.Uint64()}

	peerState.EXPECT().ReleaseUI(ui).Return(fmt.Errorf("UI already released"))
	assert.Panics(t, func() { releaseUI(replicaID, ui) })

	peerState.EXPECT().ReleaseUI(ui).Return(nil)
	assert.NotPanics(t, func() { releaseUI(replicaID, ui) })
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
