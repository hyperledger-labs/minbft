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
	"fmt"
	"strings"
	"testing"

	"github.com/hyperledger-labs/minbft/api"

	"github.com/stretchr/testify/assert"
)

var (
	testKeyStoreExamples = []struct {
		spec    string
		content string
	}{
		{"SGX_ECDSA", sgxEcdsaKeyStoreExampleYAML},
		{"ECDSA", ecdsaKeyStoreExampleYAML},
	}
	testnetKeygenCases = []*TestnetKeyOpts{
		{
			4, "ECDSA", 256,
			1, "ECDSA", 256,
			usigEnclaveFile,
		},
	}
)

const usigEnclaveFile = "../../usig/sgx/enclave/libusig.signed.so"

var sgxEcdsaKeyStoreExampleYAML = `
replica:
    keyspec: SGX_ECDSA
    keys:
        - {id: 0, privateKey: "", publicKey: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==}
        - {id: 1, privateKey: "", publicKey: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==}
        - {id: 2, privateKey: "", publicKey: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==}
client:
    keyspec: SGX_ECDSA
    keys:
        - {id: 10, privateKey: "", publicKey: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==}
`

var ecdsaKeyStoreExampleYAML = `
replica:
    keyspec: ECDSA
    keys:
        - id: 0
          privateKey: MHcCAQEEIFxopskcl2LyZ/LLsDMBfQk/82WZQI/YhvXNSYZNmUSFoAoGCCqGSM49AwEHoUQDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==
          publicKey: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==
        - id: 1
          privateKey: MHcCAQEEIFxopskcl2LyZ/LLsDMBfQk/82WZQI/YhvXNSYZNmUSFoAoGCCqGSM49AwEHoUQDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==
          publicKey: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==
        - id: 2
          privateKey: MHcCAQEEIFxopskcl2LyZ/LLsDMBfQk/82WZQI/YhvXNSYZNmUSFoAoGCCqGSM49AwEHoUQDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==
          publicKey: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==

client:
    keyspec: ECDSA
    keys:
        - id: 10
          privateKey: MHcCAQEEIFxopskcl2LyZ/LLsDMBfQk/82WZQI/YhvXNSYZNmUSFoAoGCCqGSM49AwEHoUQDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==
          publicKey: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEh6uiVdr+3EgyT3YEilvrvzQINr8eolxR22/0JudQrpGbLQQQIK+7RdnoLaIyZIlakZkb1tAws0iQN263EkwzGw==
`

func testLoadSimpleKeyStore(t *testing.T) {
	testCases := []struct {
		expectPass bool
		role       api.AuthenticationRole
		id         uint32
	}{
		{true, api.ReplicaAuthen, 0},
		{true, api.ReplicaAuthen, 1},
		{true, api.ReplicaAuthen, 2},
		{true, api.ClientAuthen, 10},
		{false, api.AuthenticationRole(-1), 0},
		{false, api.ReplicaAuthen, 4},
	}

	for _, example := range testKeyStoreExamples {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s/%s-%d", example.spec, tc.role, tc.id), func(t *testing.T) {
				ks, err := LoadSimpleKeyStore(strings.NewReader(example.content), []api.AuthenticationRole{tc.role}, tc.id)
				if tc.expectPass {
					assert.NoError(t, err, fmt.Sprintf("LoadSimpleKeyStore failed for %s-%d: %v", tc.role, tc.id, err))
					assert.NotNil(t, ks)
					assert.Equal(t, example.spec, ks.KeySpec(tc.role))
				} else {
					assert.Error(t, err, fmt.Sprintf("LoadSimpleKeyStore did not fail with unknown entry %s-%d", tc.role, tc.id))
				}
			})
		}
	}
}

func testGenerateSampleKeyFile(t *testing.T) {
	var keyConf bytes.Buffer

	for i, tc := range testnetKeygenCases {
		t.Run(fmt.Sprintf("case=%d", i), func(t *testing.T) {
			err := GenerateTestnetKeys(&keyConf, tc)
			assert.NoError(t, err)

			_, err = LoadSimpleKeyStore(&keyConf, []api.AuthenticationRole{api.ReplicaAuthen}, 0)
			assert.NoError(t, err, fmt.Sprintf("Generated key store file cannot be parsed correctly: %v", err))
		})
	}
}

func TestKeyStore(t *testing.T) {
	t.Run("LoadSimpleKeyStore", testLoadSimpleKeyStore)
	t.Run("GenerateTestnetKeys", testGenerateSampleKeyFile)
}
