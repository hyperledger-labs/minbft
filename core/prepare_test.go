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

	"github.com/stretchr/testify/assert"

	testifymock "github.com/stretchr/testify/mock"

	"github.com/hyperledger-labs/minbft/messages"
)

func TestMakePrepareValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	backup := randOtherReplicaID(primary, n)

	request := messageImpl.NewRequest(0, rand.Uint64(), nil)

	validate := makePrepareValidator(n)

	prepare := messageImpl.NewPrepare(backup, view, request)
	err := validate(prepare)
	assert.Error(t, err)

	prepare = messageImpl.NewPrepare(primary, view, request)
	err = validate(prepare)
	assert.NoError(t, err)
}

func TestMakePrepareApplier(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	view := randView()
	primary := primaryID(n, view)
	id := randOtherReplicaID(primary, n)
	prepareRequestSeq := func(request messages.Request) (new bool) {
		args := mock.MethodCalled("requestSeqPreparer", request)
		return args.Bool(0)
	}
	collectCommitment := func(msg messages.CertifiedMessage) error {
		args := mock.MethodCalled("commitmentCollector", msg)
		return args.Error(0)
	}
	handleGeneratedMessage := func(msg messages.ReplicaMessage) {
		mock.MethodCalled("generatedMessageHandler", msg)
	}
	stopPrepTimer := func(request messages.Request) {
		mock.MethodCalled("prepareTimerStopper", request)
	}
	apply := makePrepareApplier(id, prepareRequestSeq, collectCommitment, handleGeneratedMessage, stopPrepTimer)

	clientID := rand.Uint32()
	request := messageImpl.NewRequest(clientID, rand.Uint64(), nil)
	ownPrepare := messageImpl.NewPrepare(id, viewForPrimary(n, id), request)
	prepare := messageImpl.NewPrepare(primary, view, request)
	commit := messageImpl.NewCommit(id, prepare)

	mock.On("requestSeqPreparer", request).Return(false).Once()
	err := apply(prepare, true)
	assert.Error(t, err, "Request ID already prepared")

	mock.On("requestSeqPreparer", request).Return(false).Once()
	err = apply(ownPrepare, true)
	assert.Error(t, err, "Request ID already prepared")

	mock.On("requestSeqPreparer", request).Return(true).Once()
	mock.On("commitmentCollector", ownPrepare).Return(fmt.Errorf("error")).Once()
	err = apply(ownPrepare, true)
	assert.Error(t, err, "Failed to collect commitment")

	mock.On("requestSeqPreparer", request).Return(true).Once()
	mock.On("commitmentCollector", ownPrepare).Return(nil).Once()
	err = apply(ownPrepare, true)
	assert.NoError(t, err)

	mock.On("requestSeqPreparer", request).Return(true).Once()
	mock.On("commitmentCollector", prepare).Return(fmt.Errorf("error")).Once()
	err = apply(prepare, true)
	assert.Error(t, err, "Failed to collect commitment")

	mock.On("requestSeqPreparer", request).Return(true).Once()
	mock.On("commitmentCollector", prepare).Return(nil).Once()
	mock.On("prepareTimerStopper", request).Once()
	mock.On("generatedMessageHandler", commit).Once()
	err = apply(prepare, true)
	assert.NoError(t, err)

	mock.On("requestSeqPreparer", request).Return(true).Once()
	mock.On("commitmentCollector", prepare).Return(nil).Once()
	err = apply(prepare, false)
	assert.NoError(t, err)
}
