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

package sgx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	enclaveFile = "enclave/libusig.signed.so"
)

func TestSGXUSIG(t *testing.T) {
	msg := []byte("Test message")
	wrongMsg := []byte("Another message")

	// Create the first USIG instance to generate a new USIG key
	usig, err := New(enclaveFile, nil)
	assert.NoError(t, err, "Error creating fist SGXUSIG instance")
	assert.NotNil(t, usig, "Got nil SGXUSIG instance")

	key, err := usig.SealedKey()
	assert.NoError(t, err, "Error getting sealed key")
	assert.NotNil(t, key, "Got nil sealed key")

	// Get the public key from the fist instance
	usigID := usig.ID()
	assert.NotNil(t, usigID, "Got nil USIG identity")
	assert.NotEqual(t, 0, len(usigID), "Got zero-length USIG identity")

	// Destroy the fist instance, just to be sure
	usig.Destroy()

	// Recreate USIG restoring the key from the first instance
	usig, err = New(enclaveFile, key)
	assert.NoError(t, err, "Error creating SGXUSIG instance with key unsealing")
	assert.NotNil(t, usig, "Got nil SGXUSIG instance")
	defer usig.Destroy()

	ui, err := usig.CreateUI(msg)
	assert.NoError(t, err, "Error creating UI")
	assert.NotNil(t, ui, "Got nil UI")
	assert.Equal(t, uint64(1), ui.Counter, "Got wrong UI counter value")

	ui, err = usig.CreateUI(msg)
	assert.NoError(t, err, "Error creating UI")
	assert.NotNil(t, ui, "Got nil UI")
	assert.Equal(t, uint64(2), ui.Counter, "Got wrong UI counter value")

	// There's no need to repeat all the checks covered by C test
	// of the enclave. But correctness of the signature should be
	// checked.
	err = VerifyUI(msg, ui, usigID)
	assert.NoError(t, err, "Error verifying UI")

	err = VerifyUI(wrongMsg, ui, usigID)
	assert.Error(t, err, "No error verifying UI with forged message")
}
