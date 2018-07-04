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

package fake

import (
	"crypto"
	"fmt"

	"github.com/nec-blockchain/minbft/api"
	authen "github.com/nec-blockchain/minbft/sample/authentication"
	"github.com/nec-blockchain/minbft/usig"
)

//====== Fake USIG cipher ======

type fakeUsigCipher struct {
	cv uint64 // monotonic counter
}

var _ = authen.SignatureCipher(&fakeUsigCipher{})

// Sign return a UI
func (u *fakeUsigCipher) Sign(md []byte, privKey interface{}) ([]byte, error) {
	u.cv++
	ui := &usig.UI{Epoch: 42, Counter: u.cv}
	return ui.MarshalBinary()
}

// Verify always returns true
func (u *fakeUsigCipher) Verify(md, sig []byte, pubKey interface{}) bool {
	var ui usig.UI
	err := ui.UnmarshalBinary(sig)
	return err == nil
}

//====== Fake Authenticator with USIG cipher ======

type fakeUsigAuth struct {
	authScheme *authen.PublicAuthenScheme
}

var _ = api.Authenticator(&fakeUsigAuth{})

// NewFakeUsigAuthenticator returns a fake USIG authenticator that
// generate UI with virtual monotonic counter and always return true
// for verification
func NewFakeUsigAuthenticator() api.Authenticator {
	return &fakeUsigAuth{
		authScheme: &authen.PublicAuthenScheme{
			HashScheme: crypto.SHA256,
			SigCipher:  &fakeUsigCipher{}},
	}
}

// VerifyMessageAuthenTag verifies the UI
func (a *fakeUsigAuth) VerifyMessageAuthenTag(role api.AuthenticationRole, id uint32, msg []byte, authenTag []byte) error {
	if role == api.USIGAuthen {
		if err := a.authScheme.VerifyAuthenticationTag(msg, authenTag, nil); err != nil {
			return fmt.Errorf("invalid authentication tag: %v", err)
		}
	}

	return nil
}

// GenerateMessageAuthenTag generates the UI
func (a *fakeUsigAuth) GenerateMessageAuthenTag(role api.AuthenticationRole, msg []byte) ([]byte, error) {
	if role == api.USIGAuthen {
		return a.authScheme.GenerateAuthenticationTag(msg, nil)
	}

	return []byte{}, nil
}
