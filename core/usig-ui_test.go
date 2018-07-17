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

	providePeerState, msg, peerState := setupPeerStateProviderMock(ctrl, mock)

	verifyUI := func(msg messages.MessageWithUI) (*usig.UI, error) {
		args := mock.MethodCalled("uiVerifier", msg)
		return args.Get(0).(*usig.UI), args.Error(1)
	}

	captureUI := makeUICapturer(providePeerState, verifyUI)

	mock.On("uiVerifier", msg).Return((*usig.UI)(nil), fmt.Errorf("Invalid UI")).Once()
	new, err := captureUI(msg)
	assert.Error(t, err)
	assert.False(t, new)

	ui := &usig.UI{Counter: uint64(1), Cert: []byte{}}

	mock.On("uiVerifier", msg).Return(ui, nil).Once()
	peerState.EXPECT().CaptureUI(ui).Return(false)
	new, err = captureUI(msg)
	assert.NoError(t, err)
	assert.False(t, new)

	mock.On("uiVerifier", msg).Return(ui, nil).Once()
	peerState.EXPECT().CaptureUI(ui).Return(true)
	new, err = captureUI(msg)
	assert.NoError(t, err)
	assert.True(t, new)
}

func TestMakeUIReleaser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	providePeerState, msg, peerState := setupPeerStateProviderMock(ctrl, mock)

	releaseUI := makeUIReleaser(providePeerState)

	ui := &usig.UI{Counter: uint64(1), Cert: []byte{}}
	uiBytes, _ := ui.MarshalBinary()
	msg.EXPECT().UIBytes().Return(uiBytes).AnyTimes()

	peerState.EXPECT().ReleaseUI(ui).Return(fmt.Errorf("UI already released"))
	assert.Panics(t, func() { releaseUI(msg) })

	peerState.EXPECT().ReleaseUI(ui).Return(nil)
	assert.NotPanics(t, func() { releaseUI(msg) })
}

func setupPeerStateProviderMock(ctrl *gomock.Controller, mock *testifymock.Mock) (peerstate.Provider, *mock_messages.MockMessageWithUI, *mock_peerstate.MockState) {
	peerState := mock_peerstate.NewMockState(ctrl)
	msg := mock_messages.NewMockMessageWithUI(ctrl)

	providePeerState := func(replicaID uint32) peerstate.State {
		args := mock.MethodCalled("peerStateProvider", replicaID)
		return args.Get(0).(peerstate.State)
	}

	replicaID := rand.Uint32()
	msg.EXPECT().ReplicaID().Return(replicaID).AnyTimes()
	mock.On("peerStateProvider", replicaID).Return(peerState)

	return providePeerState, msg, peerState
}
