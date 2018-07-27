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

	sgxusig "github.com/nec-blockchain/minbft/usig/sgx"
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
	assert.Nil(t, err)
	ecdsaPubKey = &ecdsaPrivKey.PublicKey
}

func testEcdsaSigCipher(t *testing.T) {
	ecdsaSigCipher := &EcdsaSigCipher{}

	md := crypto.SHA256.New().Sum(testMessage)
	sig, err := ecdsaSigCipher.Sign(md, ecdsaPrivKey)
	assert.Nil(t, err, "Failed to generate ECDSA signature.")

	ok := ecdsaSigCipher.Verify(md, sig, ecdsaPubKey)
	assert.True(t, ok, "Failed to verify ECDSA signature.")
}

func testEcdsaAuthenScheme(t *testing.T) {
	ecdsaAuthScheme := &PublicAuthenScheme{crypto.SHA256, &EcdsaSigCipher{}}

	tag, err := ecdsaAuthScheme.GenerateAuthenticationTag(testMessage, ecdsaPrivKey)
	assert.Nil(t, err, "Failed to generate ECDSA authentication tag.")

	err = ecdsaAuthScheme.VerifyAuthenticationTag(testMessage, tag, ecdsaPubKey)
	assert.NoError(t, err, "Failed to verify ECDSA authentication tag.")
}

func testUSIGAuthenScheme(t *testing.T) {
	usig, err := sgxusig.New(usigEnclaveFile, nil)
	assert.NoError(t, err, "Failed to create SGX USIG")

	usigAuthScheme := NewSGXUSIGAuthenticationScheme(usig)

	tag, err := usigAuthScheme.GenerateAuthenticationTag(testMessage, nil)
	assert.NoError(t, err, "Failed to generate USIG authentication tag")

	err = usigAuthScheme.VerifyAuthenticationTag(testMessage, tag, usig.PublicKey())
	assert.NoError(t, err, "Failed to verify USIG authentication tag")
}

func TestCrypto(t *testing.T) {
	// setup
	initTestCredentials(t)

	t.Run("ecdsaSigCipher", testEcdsaSigCipher)
	t.Run("ecdsaAuthScheme", testEcdsaAuthenScheme)
	t.Run("usigAuthScheme", testUSIGAuthenScheme)
}
