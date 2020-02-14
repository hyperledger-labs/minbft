// Copyright (c) 2021 NEC Laboratories Europe GmbH.
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

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
)

func TestMakeReqViewChangeValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	verify := func(msg messages.SignedMessage) error {
		args := mock.MethodCalled("messageSignatureVerifier", msg)
		return args.Error(0)
	}
	validate := makeReqViewChangeValidator(verify)

	rvc := messageImpl.NewReqViewChange(rand.Uint32(), 0)
	err := validate(rvc)
	assert.Error(t, err, "Invalid new view number")

	rvc = messageImpl.NewReqViewChange(rand.Uint32(), 1+uint64(rand.Int63()))

	mock.On("messageSignatureVerifier", rvc).Return(fmt.Errorf("invalid signature")).Once()
	err = validate(rvc)
	assert.Error(t, err, "Invalid signature")

	mock.On("messageSignatureVerifier", rvc).Return(nil).Once()
	err = validate(rvc)
	assert.NoError(t, err)
}
