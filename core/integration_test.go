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

package minbft_test

import (
	"bytes"
	"fmt"
	"log"
	"testing"
	"text/template"
	"time"

	"github.com/hyperledger-labs/minbft/api"
	cl "github.com/hyperledger-labs/minbft/client"
	minbft "github.com/hyperledger-labs/minbft/core"
	authen "github.com/hyperledger-labs/minbft/sample/authentication"
	"github.com/hyperledger-labs/minbft/sample/config"
	"github.com/hyperledger-labs/minbft/sample/conn/common/replicastub"
	dummyConnector "github.com/hyperledger-labs/minbft/sample/conn/dummy/connector"
	"github.com/hyperledger-labs/minbft/sample/requestconsumer"

	"github.com/stretchr/testify/assert"
)

const (
	testClientID = 0

	waitDuration = 200 * time.Millisecond

	usigEnclaveFile = "../usig/sgx/enclave/libusig.signed.so"
)

type testReplicaStack struct {
	api.ReplicaConnector
	api.Authenticator
	*requestconsumer.SimpleLedger
}

type testClientStack struct {
	api.ReplicaConnector
	api.Authenticator
}

var (
	replicas      []minbft.Replica
	replicaStacks []*testReplicaStack

	clients      []cl.Client
	clientStacks []*testClientStack

	replicaStubs []replicastub.ReplicaStub

	testRequestMessage = []byte("test request message")
)

const cfgTemplate = `
protocol:
  "n": {{.N}}
  f: {{getF .N}}
  checkpointPeriod: 10
  logsize: 20
  timeout:
    request: 2s
    viewchange: 3s
`

// createTestCfg creates config file for `numReplica` replicas
func createTestCfg(numReplica int) []byte {
	var err error
	var testCfg bytes.Buffer

	t := template.New("cfgTemplate")
	t = t.Funcs(template.FuncMap{
		"getF": func(n int) int { return (n - 1) / 2 },
	})
	t = template.Must(t.Parse(cfgTemplate))
	if err = t.Execute(&testCfg, struct{ N int }{numReplica}); err != nil {
		panic(err)
	}

	return testCfg.Bytes()
}

// createTestKeys creates keystore files for `numReplica` replicas
// and `numClient` clients
func createTestKeys(numReplica, numClient int) []byte {
	var testKeys bytes.Buffer
	const testKeySpec = "ECDSA"

	if err := authen.GenerateTestnetKeys(&testKeys, &authen.TestnetKeyOpts{
		NumberReplicas:  numReplica,
		ReplicaKeySpec:  testKeySpec,
		ReplicaSecParam: 256,
		NumberClients:   numClient,
		ClientKeySpec:   testKeySpec,
		ClientSecParam:  256,
		UsigEnclaveFile: usigEnclaveFile,
	}); err != nil {
		log.Fatalf("Failed to generate testnet keys: %v", err)
	}

	return testKeys.Bytes()
}

func resetFixture() {
	replicas = nil
	replicaStacks = nil
	clients = nil
	clientStacks = nil
	replicaStubs = nil
}

func initTestnetPeers(numReplica int, numClient int) {
	resetFixture()

	// generate config, keys
	testCfg := createTestCfg(numReplica)
	testKeys := createTestKeys(numReplica, numClient)

	cfg := config.New() // configer shared by all replicas
	err := cfg.ReadConfig(bytes.NewBuffer(testCfg), "yaml")
	if err != nil {
		panic(err)
	}

	makeReplicas(numReplica, testKeys, cfg)
	makeClients(numClient, testKeys, cfg)
}

func teardownTestnet() {
	for _, client := range clients {
		client.Terminate()
	}
	for _, replica := range replicas {
		replica.Terminate()
	}
}

// Initialize a given number of replica instances.
func makeReplicas(numReplica int, testKeys []byte, cfg api.Configer) {
	// replica stubs
	for i := 0; i < numReplica; i++ {
		replicaStubs = append(replicaStubs, replicastub.New())
	}

	for i := 0; i < numReplica; i++ {
		id := uint32(i)
		sigAuth, _ := authen.NewWithSGXUSIG([]api.AuthenticationRole{api.ReplicaAuthen, api.USIGAuthen}, id, bytes.NewBuffer(testKeys), usigEnclaveFile)
		ledger := requestconsumer.NewSimpleLedger()
		conn := newReplicaSideConnector(id)
		stack := &testReplicaStack{conn, sigAuth, ledger}
		replicaStacks = append(replicaStacks, stack)

		replica, _ := minbft.New(id, cfg, stack)
		replicas = append(replicas, replica)
		replicaStubs[i].AssignReplica(replica)
	}
}

func newReplicaSideConnector(replicaID uint32) api.ReplicaConnector {
	conn := dummyConnector.NewReplicaSide()
	for i, stub := range replicaStubs {
		id := uint32(i)
		if id == replicaID {
			continue
		}
		conn.AssignReplicaStub(id, stub)
	}
	return conn
}

// Initialize a given number of client instances.
func makeClients(numClient int, testKeys []byte, cfg api.Configer) {
	for i := 0; i < numClient; i++ {
		au, _ := authen.New([]api.AuthenticationRole{api.ClientAuthen}, testClientID, bytes.NewBuffer(testKeys))
		conn := newClientSideConnector()
		stack := &testClientStack{conn, au}
		clientStacks = append(clientStacks, stack)

		client, _ := cl.New(testClientID, cfg.N(), cfg.F(), stack)
		clients = append(clients, client)
	}
}

func newClientSideConnector() api.ReplicaConnector {
	conn := dummyConnector.NewClientSide()
	for i, stub := range replicaStubs {
		conn.AssignReplicaStub(uint32(i), stub)
	}
	return conn
}

func testAcceptOneRequest(t *testing.T) {
	client := clients[0]
	<-client.Request(testRequestMessage)

	// Wait for all replicas to finish request processing; client
	// waits only for f+1 replies
	time.Sleep(waitDuration)

	for _, stack := range replicaStacks {
		assert.Equal(t, uint64(1), stack.SimpleLedger.GetLength())
	}

}

func TestIntegration(t *testing.T) {
	testCases := []struct {
		numReplica int
		numClient  int
	}{
		{numReplica: 3, numClient: 1},
		{numReplica: 5, numClient: 1},
	}
	for _, tc := range testCases {
		// setup
		initTestnetPeers(tc.numReplica, tc.numClient)

		t.Run(fmt.Sprintf("TestnetAcceptOneRequest/r=%d/c=%d", tc.numReplica, tc.numClient), testAcceptOneRequest)
	}
}
