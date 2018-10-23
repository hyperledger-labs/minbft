// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Wenting Li <wenting.li@neclab.eu>
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

package authenticator

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hyperledger-labs/minbft/api"
)

func testReplicaAuthenticator(t *testing.T, ks []byte) {
	a0, err := NewWithSGXUSIG([]api.AuthenticationRole{api.ReplicaAuthen, api.USIGAuthen}, 0, bytes.NewBuffer(ks), usigEnclaveFile)
	if !assert.NoError(t, err, "failed to create authenticator") {
		t.FailNow()
	}

	a1, err := NewWithSGXUSIG([]api.AuthenticationRole{api.ReplicaAuthen, api.USIGAuthen}, 1, bytes.NewBuffer(ks), usigEnclaveFile)
	if !assert.NoError(t, err, "failed to create authenticator") {
		t.FailNow()
	}

	tag0, err := a0.GenerateMessageAuthenTag(api.ReplicaAuthen, testMessage)
	assert.NoError(t, err, "failed to generate authentication tag")

	err = a1.VerifyMessageAuthenTag(api.ReplicaAuthen, 0, testMessage, tag0)
	assert.NoError(t, err, "verification failed")

	tag0, err = a0.GenerateMessageAuthenTag(api.USIGAuthen, testMessage)
	assert.NoError(t, err, "failed to generate authentication tag")

	err = a1.VerifyMessageAuthenTag(api.USIGAuthen, 0, testMessage, tag0)
	assert.NoError(t, err, "verification failed")
}

func testClientAuthenticator(t *testing.T, ks []byte) {
	a0, err := New([]api.AuthenticationRole{api.ClientAuthen}, 0, bytes.NewBuffer(ks))
	if !assert.NoError(t, err, "failed to create authenticator") {
		t.FailNow()
	}

	a1, err := New([]api.AuthenticationRole{api.ReplicaAuthen}, 1, bytes.NewBuffer(ks))
	if !assert.NoError(t, err, "failed to create authenticator") {
		t.FailNow()
	}

	tag0, err := a0.GenerateMessageAuthenTag(api.ClientAuthen, testMessage)
	assert.NoError(t, err, "failed to generate authentication tag")

	err = a1.VerifyMessageAuthenTag(api.ClientAuthen, 0, testMessage, tag0)
	assert.NoError(t, err, "verification failed")
}

func TestAuthenticator(t *testing.T) {
	for _, tc := range testnetKeygenCases {
		var ks bytes.Buffer
		err := GenerateTestnetKeys(&ks, tc)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		ksBytes := ks.Bytes()

		t.Run("Replica", func(t *testing.T) { testReplicaAuthenticator(t, ksBytes) })
		t.Run("Client", func(t *testing.T) { testClientAuthenticator(t, ksBytes) })
	}
}
