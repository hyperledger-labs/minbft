// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Wenting Li     <wenting.li@neclab.eu>
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

package usig_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/nec-blockchain/minbft/usig"
	mock_usig "github.com/nec-blockchain/minbft/usig/mocks"
)

func TestUIMarshalUnmarshal(t *testing.T) {
	uiIn := &usig.UI{
		Epoch:   13,
		Counter: 42,
		Cert:    []byte("test cert"),
	}
	bytes, err := uiIn.MarshalBinary()
	assert.NoError(t, err)
	uiOut := new(usig.UI)
	err = uiOut.UnmarshalBinary(bytes)
	assert.NoError(t, err)
	assert.True(t, assert.ObjectsAreEqual(uiIn, uiOut),
		"Unmarshaled UI doesn't equal marshaled")
}

// TestMockUsig shows the usage of mocked USIG
func TestMockUsig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUsig := mock_usig.NewMockUSIG(ctrl)

	uis := []usig.UI{
		{Counter: 1},
		{Counter: 2},
		{Counter: 3},
	}
	uisExp := []usig.UI{
		{Counter: 1},
		{Counter: 2},
		{Counter: 3},
	}
	gomock.InOrder(
		mockUsig.EXPECT().CreateUI(gomock.Any()).Return(&uis[0], nil),
		mockUsig.EXPECT().CreateUI(gomock.Any()).Return(&uis[1], nil),
		mockUsig.EXPECT().CreateUI(gomock.Any()).Return(&uis[2], nil),
	)
	for i := range uis {
		mockUsig.EXPECT().VerifyUI(gomock.Any(), &uis[i], gomock.Any()).Return(nil).AnyTimes()
	}

	testMsg := []byte("any message")
	ui0, err := mockUsig.CreateUI(testMsg)
	assert.NoError(t, err)
	assert.True(t, assert.ObjectsAreEqual(&uisExp[0], ui0))
	ui1, err := mockUsig.CreateUI(testMsg)
	assert.NoError(t, err)
	assert.True(t, assert.ObjectsAreEqual(&uisExp[1], ui1))
	ui2, err := mockUsig.CreateUI(testMsg)
	assert.NoError(t, err)
	assert.True(t, assert.ObjectsAreEqual(&uisExp[2], ui2))
	assert.NoError(t, mockUsig.VerifyUI(testMsg, &uis[0], []byte("usig id")))
}
