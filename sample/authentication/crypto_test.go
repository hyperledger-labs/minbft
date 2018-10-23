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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sgxusig "github.com/hyperledger-labs/minbft/usig/sgx"
)

var (
	testMessage = []byte("hello")

	ecdsaPrivKey *ecdsa.PrivateKey
	ecdsaPubKey  *ecdsa.PublicKey
)

func initTestCredentials(t *testing.T) {
	var err error

	curve := elliptic.P256()
	ecdsaPrivKey, err = ecdsa.GenerateKey(curve, rand.Reader)
	assert.NoError(t, err, "Failed to generate ECDSA key pair.")
	ecdsaPubKey = &ecdsaPrivKey.PublicKey
}

func testEcdsaSigCipher(t *testing.T) {
	ecdsaSigCipher := &EcdsaSigCipher{}

	md := crypto.SHA256.New().Sum(testMessage)
	sig, err := ecdsaSigCipher.Sign(md, ecdsaPrivKey)
	assert.NoError(t, err, "Failed to generate ECDSA signature.")

	ok := ecdsaSigCipher.Verify(md, sig, ecdsaPubKey)
	assert.True(t, ok, "Failed to verify ECDSA signature.")
}

func testEcdsaAuthenScheme(t *testing.T) {
	ecdsaAuthScheme := &PublicAuthenScheme{crypto.SHA256, &EcdsaSigCipher{}}

	tag, err := ecdsaAuthScheme.GenerateAuthenticationTag(testMessage, ecdsaPrivKey)
	assert.NoError(t, err, "Failed to generate ECDSA authentication tag.")

	err = ecdsaAuthScheme.VerifyAuthenticationTag(testMessage, tag, ecdsaPubKey)
	assert.NoError(t, err, "Failed to verify ECDSA authentication tag.")
}

func testUSIGAuthenScheme(t *testing.T) {
	usig1, err := sgxusig.New(usigEnclaveFile, nil)
	require.NoError(t, err)

	pubKey := usig1.PublicKey()
	usigAuthScheme1 := NewSGXUSIGAuthenticationScheme(usig1)

	tag1, err := usigAuthScheme1.GenerateAuthenticationTag(testMessage, nil)
	require.NoError(t, err)

	err = usigAuthScheme1.VerifyAuthenticationTag(testMessage, tag1, pubKey)
	assert.NoError(t, err)

	if testing.Short() {
		t.Skip("skipping USIG enclave recreation in short mode.")
	}

	usig2, err := sgxusig.New(usigEnclaveFile, usig1.SealedKey())
	require.NoError(t, err)
	assert.Equal(t, pubKey, usig2.PublicKey())

	usigAuthScheme2 := NewSGXUSIGAuthenticationScheme(usig2)

	tag2, err := usigAuthScheme2.GenerateAuthenticationTag(testMessage, nil)
	require.NoError(t, err)

	err = usigAuthScheme1.VerifyAuthenticationTag(testMessage, tag2, pubKey)
	assert.Error(t, err)
}

func TestCrypto(t *testing.T) {
	// setup
	initTestCredentials(t)

	t.Run("ecdsaSigCipher", testEcdsaSigCipher)
	t.Run("ecdsaAuthScheme", testEcdsaAuthenScheme)
	t.Run("usigAuthScheme", testUSIGAuthenScheme)
}
