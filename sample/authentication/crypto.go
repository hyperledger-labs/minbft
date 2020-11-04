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
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"math/big"
	"sync"

	"github.com/hyperledger-labs/minbft/usig"
	sgxusig "github.com/hyperledger-labs/minbft/usig/sgx"
)

// SignatureCipher defines the interface of signature operations used by public cryptographic ciphers
type SignatureCipher interface {
	// Sign creates signature over the message digest
	Sign(md []byte, privKey interface{}) ([]byte, error)
	// Verify verifies the signature over the message digest
	Verify(md, sig []byte, pubKey interface{}) bool
}

//========= SignatureCipher implementations =======

type (
	// EcdsaNIST256pSigCipher implements the SignatureCipher interface with
	// signature scheme EcdsaNIST256p
	EcdsaNIST256pSigCipher struct{}
)

// EcdsaSigCipher is alias to EcdsaNIST256pSigCipher
type EcdsaSigCipher EcdsaNIST256pSigCipher

// type assertions for interface impl.
var _ = SignatureCipher(&EcdsaSigCipher{})

// ecdsaSignature gives the ASN.1 encoding of the signature
type ecdsaSignature struct {
	R, S *big.Int
}

// Sign returns an ECDSA signature that is encoded as ASN.1 der format
func (c *EcdsaSigCipher) Sign(md []byte, privKey interface{}) ([]byte, error) {
	if eccPrivKey, ok := privKey.(*ecdsa.PrivateKey); ok {
		r, s, err := ecdsa.Sign(rand.Reader, eccPrivKey, md)
		if err != nil {
			return nil, fmt.Errorf("ECDSA signing error: %v", err)
		}
		sig, err := asn1.Marshal(ecdsaSignature{r, s})
		if err != nil {
			return nil, fmt.Errorf("ECDSA signature ASN1-DER marshal error: %v", err)
		}
		return sig, nil
	}
	return nil, fmt.Errorf("incompatible format of ECDSA private key")
}

// Verify verifies a ECDSA signature that is encoded as ASN.1 der format
func (c *EcdsaSigCipher) Verify(md, sig []byte, pubKey interface{}) bool {
	ecdsaSig := &ecdsaSignature{}
	_, err := asn1.Unmarshal(sig, ecdsaSig)
	if err != nil {
		panic(fmt.Sprintf("ECDSA signature is not ASN.1-DER encoded: %v", err))
	}
	if ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey); ok {
		return ecdsa.Verify(ecdsaPubKey, md, ecdsaSig.R, ecdsaSig.S)
	}
	return false
}

//=========== Authentication Schemes ============

// AuthenticationScheme defines an interface to create/verify
// authentication tags of any arbitrary messages
type AuthenticationScheme interface {
	GenerateAuthenticationTag(m []byte, privKey interface{}) ([]byte, error)
	VerifyAuthenticationTag(m []byte, sig []byte, pubKey interface{}) error
}

// PublicAuthenScheme specifies the adopted public authentication scheme. It
// defines a hash scheme and a signature scheme to create/verify authentication
// tags of any arbitrary messages
type PublicAuthenScheme struct {
	HashScheme crypto.Hash
	SigCipher  SignatureCipher
}

var _ AuthenticationScheme = (*PublicAuthenScheme)(nil)

// GenerateAuthenticationTag returns the signature on the message as the
// authentication tag. The digest of the message is first computed with
// specified hash scheme before signing
func (a *PublicAuthenScheme) GenerateAuthenticationTag(m []byte, privKey interface{}) ([]byte, error) {
	md := a.HashScheme.New().Sum(m)
	return a.SigCipher.Sign(md, privKey)
}

// VerifyAuthenticationTag returns true if the verification is successful on
// the signature of the message.
func (a *PublicAuthenScheme) VerifyAuthenticationTag(m []byte, sig []byte, pubKey interface{}) error {
	md := a.HashScheme.New().Sum(m)
	if !a.SigCipher.Verify(md, sig, pubKey) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// usigKeyFingerprint is the first 8 bytes of SHA256 hash over the
// USIG public key.
type usigKeyFingerprint [8]byte

// makeUSIGKeyFingerprint calculates USIG fingerprint from a serialized
// USIG public key.
func makeUSIGKeyFingerprint(pubKey interface{}) (fingerprint usigKeyFingerprint, err error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return usigKeyFingerprint{}, err
	}

	pubKeyHash := sha256.Sum256(pubKeyBytes)
	copy(fingerprint[:], pubKeyHash[:])

	return fingerprint, nil
}

// SGXUSIGAuthenticationScheme impelements AuthenticationScheme interface
// by utilizing SGX USIG to create/verify authentication tags.
type SGXUSIGAuthenticationScheme struct {
	usig *sgxusig.USIG

	// USIG key fingerprint -> captured epoch value
	epoch map[usigKeyFingerprint]uint64
	lock  sync.Mutex
}

var _ AuthenticationScheme = (*SGXUSIGAuthenticationScheme)(nil)

// NewSGXUSIGAuthenticationScheme creates a new instance of SGX USIG
// authentication scheme.
func NewSGXUSIGAuthenticationScheme(usig *sgxusig.USIG) *SGXUSIGAuthenticationScheme {
	return &SGXUSIGAuthenticationScheme{
		usig:  usig,
		epoch: make(map[usigKeyFingerprint]uint64),
	}
}

// GenerateAuthenticationTag creates a new authentication for the
// message. Marshaled USIG UI represents an authentication tag.
// Supplied private key is ignored.
func (au *SGXUSIGAuthenticationScheme) GenerateAuthenticationTag(m []byte, privKey interface{}) ([]byte, error) {
	ui, err := au.usig.CreateUI(m)
	if err != nil {
		return nil, fmt.Errorf("failed to create UI: %v", err)
	}

	usigBytes, err := ui.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return usigBytes, nil
}

// VerifyAuthenticationTag verifies the supplied authentication tag.
// Marshaled USIG UI represents an authentication tag.
func (au *SGXUSIGAuthenticationScheme) VerifyAuthenticationTag(m []byte, sig []byte, pubKey interface{}) error {
	var ui usig.UI

	if err := ui.UnmarshalBinary(sig); err != nil {
		return fmt.Errorf("failed to unmarshal UI: %v", err)
	}

	fingerprint, err := makeUSIGKeyFingerprint(pubKey)
	if err != nil {
		return fmt.Errorf("failed to calculate USIG key fingerprint: %s", err)
	}

	au.lock.Lock()
	defer au.lock.Unlock()

	// Capture the epoch value received with the first valid UI to
	// dynamically determine the full USIG identity. Note that
	// this relies on the assumption that all peer replicas are
	// initially correct, each uses a unique USIG key pairs per
	// consensus protocol instance and generates its first UI
	// using a single USIG instance per replica. Moreover, those
	// first UIs are assumed to be received and processed by
	// correct replicas before any replica becomes faulty in a
	// sense that it starts generating and sending UIs using
	// another USIG instance initialized with the same sealed key
	// pair.
	//
	// Those assumptions might be too strong in some environments.
	// In that case, all correct replicas are required to use some
	// other mechanism to agree on a single USIG instance identity
	// per each replica and use that identity to verify received
	// UIs. This, for example, can be achieved using some
	// bootstrapping procedure.
	epoch, ok := au.epoch[fingerprint]
	if !ok && ui.Counter == uint64(1) {
		epoch, _, err = sgxusig.ParseCert(ui.Cert)
		if err != nil {
			return fmt.Errorf("failed to parse UI certificate: %s", err)
		}
	}

	usigID, err := sgxusig.MakeID(epoch, pubKey)
	if err != nil {
		return fmt.Errorf("failed to construct USIG identity: %s", err)
	}

	if err := au.usig.VerifyUI(m, &ui, usigID); err != nil {
		return err
	}

	au.epoch[fingerprint] = epoch

	return nil
}
