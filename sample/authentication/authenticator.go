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
	"fmt"
	"io"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/usig"
	sgxusig "github.com/hyperledger-labs/minbft/usig/sgx"
)

//========== Authenticator implementations ==========

// Authenticator defines the basic properties of an authenticator
type Authenticator struct {
	id uint32

	ks          BftKeyStorer
	authschemes map[api.AuthenticationRole]AuthenticationScheme
}

var _ api.Authenticator = (*Authenticator)(nil)

// New returns initialized authenticator
func New(roles []api.AuthenticationRole, id uint32, keystoreFileReader io.Reader) (*Authenticator, error) {
	ks, err := LoadSimpleKeyStore(keystoreFileReader, roles, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load keystore: %v", err)
	}

	au, err := new(roles, id, ks, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %v", err)
	}

	return au, nil
}

// NewWithSGXUSIG initialized replica authenticator with support of
// USIGAuthen role by using an instance of SGX USIG
func NewWithSGXUSIG(roles []api.AuthenticationRole, id uint32, keystoreFileReader io.Reader, enclaveFile string) (*Authenticator, error) {
	ks, err := LoadSimpleKeyStore(keystoreFileReader, roles, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load keystore: %v", err)
	}

	usigKey := ks.ownerKeys[api.USIGAuthen].privateKey
	if usigKey == nil {
		return nil, fmt.Errorf("failed to get USIG sealed key")
	}

	usig, err := sgxusig.New(enclaveFile, usigKey.([]byte))
	if err != nil {
		return nil, fmt.Errorf("failed to create USIG: %v", err)
	}

	au, err := new(roles, id, ks, usig)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %v", err)
	}

	return au, nil
}

// NewWithUSIG initializes authenticator with support of USIGAuthen role
func NewWithUSIG(roles []api.AuthenticationRole, id uint32, ks BftKeyStorer, usig usig.USIG) (*Authenticator, error) {
	return new(roles, id, ks, usig)
}

func new(roles []api.AuthenticationRole, id uint32, ks BftKeyStorer, usig usig.USIG) (*Authenticator, error) {
	a := &Authenticator{
		id:          id,
		ks:          ks,
		authschemes: make(map[api.AuthenticationRole]AuthenticationScheme),
	}
	for _, r := range ks.NodeRoles() {
		ksName := ks.NodeKeySpec(r)
		if (r == api.USIGAuthen) != (ksName == keySpecSgxEcdsa) {
			return nil, fmt.Errorf("cannot use %s keyspec for %s role", ksName, r)
		}
		switch ksName {
		case keySpecEcdsa:
			a.authschemes[r] = &PublicAuthenScheme{crypto.SHA256, &EcdsaSigCipher{}}
		case keySpecSgxEcdsa:
			if usig == nil {
				continue
			}
			sgxUSIG, ok := usig.(*sgxusig.USIG)
			if !ok {
				return nil, fmt.Errorf("cannot use supplied USIG: %s keyspec requires SGX USIG", ksName)
			}
			a.authschemes[r] = NewSGXUSIGAuthenticationScheme(sgxUSIG)
		default:
			return nil, fmt.Errorf("cannot find an authentication scheme corresponding to the keyspec '%s'", ks.KeySpec(r))
		}
	}
	return a, nil
}

// VerifyMessageAuthenTag verifies a message authentication tag
// produced with GenerateMessageAuthenTag on the specified
// replica/client node
func (a *Authenticator) VerifyMessageAuthenTag(role api.AuthenticationRole, id uint32, msg []byte, authenTag []byte) error {
	pk, err := a.ks.NodePublicKey(role, id)
	if err != nil {
		return err
	}
	authscheme := a.authschemes[role]
	if authscheme == nil {
		return fmt.Errorf("unknown role: %v", role)
	}
	if err := authscheme.VerifyAuthenticationTag(msg, authenTag, pk); err != nil {
		return fmt.Errorf("invalid authentication tag: %v", err)
	}
	return nil
}

// GenerateMessageAuthenTag generates message authentication tag to be
// verified by other nodes with VerifyAuthenticationTag
func (a *Authenticator) GenerateMessageAuthenTag(role api.AuthenticationRole, msg []byte) ([]byte, error) {
	sk := a.ks.PrivateKey(role)
	return a.authschemes[role].GenerateAuthenticationTag(msg, sk)
}
