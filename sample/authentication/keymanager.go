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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/hyperledger-labs/minbft/api"
	sgxusig "github.com/hyperledger-labs/minbft/usig/sgx"

	yaml "gopkg.in/yaml.v2"
)

//================ KeyStore ===============

// BftKeyStorer manages the keys for node communication
type BftKeyStorer interface {
	KeySpec(role api.AuthenticationRole) string
	PrivateKey(role api.AuthenticationRole) interface{}
	PublicKey(role api.AuthenticationRole) interface{}

	NodePublicKey(role api.AuthenticationRole, id uint32) (interface{}, error)
	NodeRoles() []api.AuthenticationRole
	NodeKeySpec(role api.AuthenticationRole) string
}

// SimpleKeyStore implements BftKeyStorer with simple maps
type SimpleKeyStore struct {
	id             uint32
	ownerKeys      map[api.AuthenticationRole]*ownerKey
	nodePublicKeys map[api.AuthenticationRole]*publicKeySet // public keys of the other nodes
}

type ownerKey struct {
	keyspec    string
	privateKey interface{} // private key of the node
	publicKey  interface{} // public key of the node

}

type publicKeySet struct {
	keyspec string
	keys    map[uint32]interface{}
}

//KeySpec returns the keyspec of the owner
func (ks *SimpleKeyStore) KeySpec(role api.AuthenticationRole) string {
	ownerKey := ks.ownerKeys[role]
	if ownerKey == nil {
		return ""
	}
	return ownerKey.keyspec
}

//PrivateKey returns the private key of the node
func (ks *SimpleKeyStore) PrivateKey(role api.AuthenticationRole) interface{} {
	ownerKey := ks.ownerKeys[role]
	if ownerKey == nil {
		return nil
	}
	return ownerKey.privateKey
}

//PublicKey returns the public key of the node
func (ks *SimpleKeyStore) PublicKey(role api.AuthenticationRole) interface{} {
	ownerKey := ks.ownerKeys[role]
	if ownerKey == nil {
		return nil
	}
	return ownerKey.publicKey
}

// NodePublicKey returns the public key of a node given his role and id
func (ks *SimpleKeyStore) NodePublicKey(role api.AuthenticationRole, id uint32) (interface{}, error) {
	if keymap := ks.nodePublicKeys[role]; keymap != nil {
		return keymap.keys[id], nil
	}
	return nil, fmt.Errorf("key set not found for role=%v, id=%d", role, id)
}

// NodeRoles returns a slice of all node roles present in the key store
func (ks *SimpleKeyStore) NodeRoles() []api.AuthenticationRole {
	var roles []api.AuthenticationRole

	for r := range ks.nodePublicKeys {
		roles = append(roles, r)
	}

	return roles
}

// NodeKeySpec return the keyspec the specified role
func (ks *SimpleKeyStore) NodeKeySpec(role api.AuthenticationRole) string {
	keymap := ks.nodePublicKeys[role]
	if keymap == nil {
		return ""
	}
	return keymap.keyspec
}

//================ Parse keystore from keystore file ===============

// simpleKeyStoreFile (combines with keySet/keyPair) follows the key store file format that
// can directly get parsed through unmarshalling.
//
// Sample key store file in yaml
//
// replica:
//   keyspec: ECDSA
//   keys:
//     - {id: 0, privatekey: ..., publickey: ... }
//     - {id: 1, privatekey: ..., publickey: ... }
//     - ...
// usig:
//   keyspec: SGX_ECDSA
//   keys:
//     - {id: 0, privatekey: ..., publickey: ... }
//     - {id: 1, privatekey: ..., publickey: ... }
//     - ...
// client:
//   keyspec: ECDSA
//   keys:
//     - {id: 0, privatekey: ..., publickey: ... }
//     - ...
type simpleKeyStoreFile struct {
	Replica *keySet `yaml:"replica"`
	Usig    *keySet `yaml:"usig"`
	Client  *keySet `yaml:"client"`
}

type keySet struct {
	KeySpec string     `yaml:"keyspec"`
	Keys    []*keyPair `yaml:"keys"`
}

type keyPair struct {
	ID         uint32 `yaml:"id"`
	PrivateKey string `yaml:"privateKey"`
	PublicKey  string `yaml:"publicKey"`
}

// parseKeyStoreFile parse the key config file and return the struct. Return nil if parse fails.
func parseSimpleKeyStoreFile(reader io.Reader) (*simpleKeyStoreFile, error) {
	fileBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read error: %v", err)
	}
	keyStoreFile := &simpleKeyStoreFile{}
	if err := yaml.Unmarshal(fileBytes, keyStoreFile); err != nil {
		return nil, fmt.Errorf("yaml parse error: %v", err)
	}
	return keyStoreFile, nil
}

// LoadSimpleKeyStore parses the key file and load the keyStore. It locates its filtering the
// config according to the role (replica/client) and the node id.
func LoadSimpleKeyStore(keystoreFileReader io.Reader, roles []api.AuthenticationRole, id uint32) (*SimpleKeyStore, error) {
	ks := &SimpleKeyStore{
		id:             id,
		ownerKeys:      make(map[api.AuthenticationRole]*ownerKey),
		nodePublicKeys: make(map[api.AuthenticationRole]*publicKeySet),
	}
	keys, err := parseSimpleKeyStoreFile(keystoreFileReader)
	if err != nil {
		return nil, err
	}

	keySets := map[api.AuthenticationRole]*keySet{
		api.ReplicaAuthen: keys.Replica,
		api.USIGAuthen:    keys.Usig,
		api.ClientAuthen:  keys.Client,
	}

	for _, r := range roles {
		ks.ownerKeys[r] = &ownerKey{}
	}

	for role, keyset := range keySets {
		if keyset == nil {
			continue
		}
		keyspec, err := getKeySpec(keyset.KeySpec)
		if err != nil {
			return nil, err
		}
		ks.nodePublicKeys[role] = &publicKeySet{
			keyspec: keyspec.getSpecName(),
			keys:    make(map[uint32]interface{}),
		}
		for _, keypair := range keyset.Keys {
			err := ks.parseKeyPair(role, keyspec, keypair)
			if err != nil {
				return nil, err
			}
		}
	}

	for _, r := range roles {
		if ks.PrivateKey(r) == nil {
			return nil, fmt.Errorf("missing key: cannot find node's own key "+
				"(role=%v, id=%d)", r, id)
		}
	}
	return ks, nil
}

func (ks *SimpleKeyStore) parseKeyPair(role api.AuthenticationRole, keyspec keySpec, keypair *keyPair) error {
	pubkeyMap := ks.nodePublicKeys[role].keys

	pubKey, err := keyspec.parsePublicKey(keypair.PublicKey)
	if err != nil {
		return err
	}
	pubkeyMap[keypair.ID] = pubKey

	ownerKey := ks.ownerKeys[role]
	if ownerKey != nil && keypair.ID == ks.id {
		privKey, err := keyspec.parsePrivateKey(keypair.PrivateKey)
		if err != nil {
			return err
		}
		ownerKey.keyspec = keyspec.getSpecName()
		ownerKey.publicKey = pubkeyMap[keypair.ID]
		ownerKey.privateKey = privKey
	}

	return nil
}

//================ Key Spec ===============

// key spec names
const (
	keySpecSgxEcdsa = "SGX_ECDSA"
	keySpecEcdsa    = "ECDSA"
)

// keySpec defines the interfaces how a (public/private) key spec can be parsed from/to key store file
type keySpec interface {
	// return the key spec name
	getSpecName() string
	// return the private key parsed from the key string in the key store file
	parsePrivateKey(privkeyStr string) (interface{}, error)
	// return the public key parsed from the key string in the key store file
	parsePublicKey(pubkeyStr string) (interface{}, error)

	// generateKeyPair generates a key pair and returns the encoding of the
	// keys that can also be parsed correctly by the ParseXXXKey interface
	generateKeyPair(securityParam int) (privKeyStr string, pubKeyStr string, err error)
}

//================ Key Spec Implementations ===============

func getKeySpec(keySpecStr string) (keySpec, error) {
	switch keySpecStr {
	case keySpecSgxEcdsa:
		return &sgxEcdsaKeySpec{}, nil
	case keySpecEcdsa:
		return &ecdsaKeySpec{}, nil
	default:
		return nil, fmt.Errorf("unknown KeySpec: %s", keySpecStr)
	}
}

//###### SGX_ECDSA #######

type sgxEcdsaKeySpec struct {
	ecdsaKeySpec
	// path to the USIG enclave image file; used only for key generation
	enclaveFile string
}

func (spec *sgxEcdsaKeySpec) getSpecName() string {
	return keySpecSgxEcdsa
}

// parsePrivateKey parses Base64-encoded sealed key pair of an SGX
// USIG enclave back into a slice of bytes
func (spec *sgxEcdsaKeySpec) parsePrivateKey(privKeyStr string) (interface{}, error) {
	sealedKey, err := base64.StdEncoding.DecodeString(privKeyStr)
	if err != nil {
		return nil, err
	}
	return sealedKey, nil
}

// generateKeyPair creates an SGX USIG instance to generate a key
// pair, where the private key is sealed by the enclave.
func (spec *sgxEcdsaKeySpec) generateKeyPair(securityParam int) (string, string, error) {
	usig, err := sgxusig.New(spec.enclaveFile, nil)
	if err != nil {
		return "", "", err
	}
	defer usig.Destroy()

	privKeyBytes := usig.SealedKey()
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(usig.PublicKey())
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal USIG public key: %v", err)
	}

	privKeyBase64 := base64.StdEncoding.EncodeToString(privKeyBytes)
	pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKeyBytes)

	return privKeyBase64, pubKeyBase64, nil
}

//###### ECDSA #######

type ecdsaKeySpec struct{}

func (spec *ecdsaKeySpec) getSpecName() string {
	return keySpecEcdsa
}

// parsePrivateKey parse the ECDSA private key in ASN.1 format encoded in base64
func (spec *ecdsaKeySpec) parsePrivateKey(privKeyStr string) (interface{}, error) {
	der, err := base64.StdEncoding.DecodeString(privKeyStr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error (ECDSA private key): %v", err)
	}
	privKey, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse error (ECDSA private key): %v", err)
	}
	return privKey, nil
}

// parsePublicKey parse a DER encoded ECDSA public key in base64
func (spec *ecdsaKeySpec) parsePublicKey(pubKeyStr string) (interface{}, error) {
	der, err := base64.StdEncoding.DecodeString(pubKeyStr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error (ECDSA public key): %v", err)
	}
	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse error (ECDSA public Key): %v", err)
	}
	pubKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key format error: expect ECDSA")
	}
	return pubKey, nil
}

func (spec *ecdsaKeySpec) generateKeyPair(securityParam int) (string, string, error) {
	var curve elliptic.Curve
	switch securityParam {
	case 224:
		curve = elliptic.P224()
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default:
		// same curve used by SGX
		curve = elliptic.P256()
	}
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return "", "", err
	}
	privKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		panic(fmt.Sprintf("x509.MarshalECPrivateKey failed: %v", err))
	}
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		panic(fmt.Sprintf("x509.MarshalPKIXPublicKey failed: %v", err))
	}

	privKeyBase64 := base64.StdEncoding.EncodeToString(privKeyBytes)
	pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKeyBytes)

	return privKeyBase64, pubKeyBase64, nil
}

//============= Helpers ==============

// TestnetKeyOpts options to supply to GenerateTestnetKeys()
type TestnetKeyOpts struct {
	NumberReplicas  int
	ReplicaKeySpec  string
	ReplicaSecParam int

	NumberClients  int
	ClientKeySpec  string
	ClientSecParam int

	UsigEnclaveFile string
}

// GenerateTestnetKeys creates a keystore configuration corresponding
// to simpleKeyStoreFile struct. YAML representation of keystore
// configuration will be written to the supplied Writer interface
func GenerateTestnetKeys(w io.Writer, opts *TestnetKeyOpts) error {
	keys := &simpleKeyStoreFile{}

	rKeySet, err := generateKeySet(opts.NumberReplicas, opts.ReplicaKeySpec,
		opts.ReplicaSecParam, "")
	if err != nil {
		return err
	}
	keys.Replica = rKeySet

	uKeySet, err := generateKeySet(opts.NumberReplicas, keySpecSgxEcdsa,
		0, opts.UsigEnclaveFile)
	if err != nil {
		return err
	}
	keys.Usig = uKeySet

	cKeySet, err := generateKeySet(opts.NumberClients, opts.ClientKeySpec,
		opts.ClientSecParam, "")
	if err != nil {
		return err
	}
	keys.Client = cKeySet

	enc := yaml.NewEncoder(w)
	err = enc.Encode(keys)
	if err != nil {
		return fmt.Errorf("failed to marshal simpleKeyStoreFile to yaml: %v", err)
	}
	return enc.Close()
}

func generateKeySet(nrKeys int, keySpec string, secParam int, enclaveFile string) (*keySet, error) {
	keyset := &keySet{
		KeySpec: keySpec,
		Keys:    make([]*keyPair, nrKeys),
	}

	spec, err := getKeySpec(keySpec)
	if err != nil {
		return nil, err
	}

	if sgxSpec, ok := spec.(*sgxEcdsaKeySpec); ok {
		sgxSpec.enclaveFile = enclaveFile
	}

	for id := range keyset.Keys {
		var privKey, pubKey string
		privKey, pubKey, err = spec.generateKeyPair(secParam)
		if err != nil {
			return nil, err
		}
		keyset.Keys[id] = &keyPair{
			ID:         uint32(id),
			PrivateKey: privKey,
			PublicKey:  pubKey,
		}
	}

	return keyset, nil
}
