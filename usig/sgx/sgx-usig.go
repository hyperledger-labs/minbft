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

// Package sgx provides Go interface to SGX USIG implementation.
package sgx

import (
	"crypto/sha256"
	"crypto/x509"
	"fmt"

	"github.com/nec-blockchain/minbft/usig"
)

// USIG implements USIG interface around USIGEnclave.
type USIG struct {
	*USIGEnclave
}

var _ usig.USIG = new(USIG)

// New creates a new instance of SGXUSIG. It is a wrapper around
// NewUSIGEnclave(). See NewUSIGEnclave() for more details. Note that
// the created instance has to be disposed with Destroy() method, e.g.
// using defer.
func New(enclaveFile string, sealedKey []byte) (*USIG, error) {
	enclave, err := NewUSIGEnclave(enclaveFile, sealedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create USIG enclave: %v", err)
	}

	return &USIG{enclave}, nil
}

// CreateUI creates a unique identifier assigned to the message.
func (u *USIG) CreateUI(message []byte) (*usig.UI, error) {
	counter, signature, err := u.USIGEnclave.CreateUI(messageDigest(message))
	if err != nil {
		return nil, err
	}

	return &usig.UI{
		Epoch:   u.Epoch(),
		Counter: counter,
		Cert:    signature,
	}, nil
}

// VerifyUI is just a wrapper around the VerifyUI function at the
// package-level.
func (u *USIG) VerifyUI(message []byte, ui *usig.UI, usigID []byte) error {
	return VerifyUI(message, ui, usigID)
}

// ID returns the SGXUSIG instance identity which is ASN.1 marshaled
// public key of the enclave.
func (u *USIG) ID() []byte {
	bytes, err := x509.MarshalPKIXPublicKey(u.PublicKey())
	if err != nil {
		panic(err)
	}

	return bytes
}

// VerifyUI verifies unique identifier generated for the message by
// USIG with the specified identity.
func VerifyUI(message []byte, ui *usig.UI, usigID []byte) error {
	pubKey, err := x509.ParsePKIXPublicKey(usigID)
	if err != nil {
		return fmt.Errorf("failed to parse USIG ID: %v", err)
	}

	return VerifySignature(pubKey, messageDigest(message), ui.Epoch, ui.Counter, ui.Cert)
}

func messageDigest(message []byte) Digest {
	return sha256.Sum256(message)
}
