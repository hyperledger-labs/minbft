// Copyright (c) 2022 NEC Laboratories Europe GmbH.
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
	. "github.com/hyperledger-labs/minbft/messages/testing"
)

func TestMakeNewViewValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	n := randN()
	v := randOtherView(0)
	p := primaryID(n, v)

	validateNVCert := func(p uint32, v uint64, c messages.NewViewCert) error {
		args := mock.MethodCalled("newViewCertValidator", p, v, c)
		return args.Error(0)
	}
	validate := makeNewViewValidator(n, validateNVCert)

	nvCert := MakeTestNVCert(messageImpl)
	nv := messageImpl.NewNewView(p, v, nvCert)

	mock.On("newViewCertValidator", p, v, nvCert).Return(fmt.Errorf("error")).Once()
	err := validate(nv)
	assert.Error(t, err, "Invalid new-view cert")

	mock.On("newViewCertValidator", p, v, nvCert).Return(nil).Once()
	err = validate(nv)
	assert.NoError(t, err)

	nv2 := messageImpl.NewNewView(0, 0, nvCert)
	mock.On("newViewCertValidator", 0, 0, nvCert).Return(nil).Maybe()
	err = validate(nv2)
	assert.Error(t, err, "Invalid new view number")

	r := randOtherReplicaID(p, n)
	nv3 := messageImpl.NewNewView(r, v, nvCert)
	mock.On("newViewCertValidator", r, v, nvCert).Return(nil).Maybe()
	err = validate(nv3)
	assert.Error(t, err, "NewView from backup replica")
}

func TestMakeNewViewCertValidator(t *testing.T) {
	mock := new(testifymock.Mock)
	defer mock.AssertExpectations(t)

	const n, f = 3, 1
	const viewChangeCertSize = n - f

	v := randOtherView(0)
	p := primaryID(n, v)

	validate := makeNewViewCertValidator(viewChangeCertSize)

	vcs := make([]messages.ViewChange, n)
	for r := range vcs {
		vcs[r] = MakeTestVC(messageImpl, uint32(r), v, nil, nil, rand.Uint64())
	}
	cert := messages.NewViewCert{vcs[p], vcs[randOtherReplicaID(p, n)]}

	err := validate(primaryID(n, v+1), v+1, cert)
	assert.Error(t, err, "Wrong view number")

	err = validate(p, v, cert[:viewChangeCertSize-1])
	assert.Error(t, err, "Insufficient quorum")

	err = validate(p, v, append(messages.NewViewCert{cert[0], cert[0]}, cert[2:]...))
	assert.Error(t, err, "Duplicate message in certificate")

	err = validate(p, v, messages.NewViewCert{vcs[primaryID(n, v+1)], vcs[primaryID(n, v+2)]})
	assert.Error(t, err, "Missing ViewChange from new primary")

	err = validate(p, v, cert)
	assert.NoError(t, err)
}
