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
	dummyConnector "github.com/hyperledger-labs/minbft/sample/net/dummy/connector"
	"github.com/hyperledger-labs/minbft/sample/requestconsumer"
	logging "github.com/op/go-logging"

	"github.com/stretchr/testify/assert"
)

const (
	testClientID = 0

	waitDuration = 200 * time.Millisecond
)

type testReplicaStack struct {
	*dummyConnector.ReplicaConnector
	api.Authenticator
	api.ProtocolHandler
	*requestconsumer.SimpleLedger
}

type testClientStack struct {
	*dummyConnector.ReplicaConnector
	api.Authenticator
}

var (
	replicas          []*minbft.Replica
	replicaConnectors []*dummyConnector.ReplicaConnector
	replicaStacks     []*testReplicaStack
	clients           []cl.Client
	clientConnectors  []*dummyConnector.ReplicaConnector
	clientStacks      []*testClientStack

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

const usigEnclaveFile = "../usig/sgx/enclave/libusig.signed.so"

// createTestnetCfgFiles create config file and keystore files for `numReplica`
// replicas and 1 client
func createTestnetCfg(numReplica int, numClient int) ([]byte, []byte) {
	var err error
	var testCfg bytes.Buffer
	var testKeys bytes.Buffer

	t := template.New("cfgTemplate")
	t = t.Funcs(template.FuncMap{
		"getF": func(n int) int { return (n - 1) / 2 },
	})
	t = template.Must(t.Parse(cfgTemplate))
	if err = t.Execute(&testCfg, struct{ N int }{numReplica}); err != nil {
		panic(err)
	}

	const testKeySpec = "ECDSA"
	if err = authen.GenerateTestnetKeys(&testKeys, &authen.TestnetKeyOpts{
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

	return testCfg.Bytes(), testKeys.Bytes()
}

func resetFixture() {
	replicas = nil
	replicaConnectors = nil
	replicaStacks = nil
	replicaStacks = nil
	clients = nil
	clientConnectors = nil
	clientStacks = nil
}

func initTestnetPeers(numReplica int, numClient int) {
	resetFixture()

	// generate config, keys
	testCfg, testKeys := createTestnetCfg(numReplica, numClient)

	cfg := config.New() // configer shared by all replicas
	err := cfg.ReadConfig(bytes.NewBuffer(testCfg), "yaml")
	if err != nil {
		panic(err)
	}

	replicaConnectors = createReplicaConnectors(numReplica, numReplica)
	clientConnectors = createReplicaConnectors(numReplica, numClient)

	logger := logging.MustGetLogger("minbft")

	// replicas
	for i := 0; i < numReplica; i++ {
		id := uint32(i)
		sigAuth, _ := authen.NewWithSGXUSIG([]api.AuthenticationRole{api.ReplicaAuthen, api.USIGAuthen}, id, bytes.NewBuffer(testKeys), usigEnclaveFile)
		ledger := requestconsumer.NewSimpleLedger()
		handler := minbft.NewMinBFTHandler(0, cfg, replicaConnectors[i], sigAuth, ledger, logger)

		replicaStacks = append(replicaStacks, &testReplicaStack{replicaConnectors[i], sigAuth, handler, ledger})

		replica, _ := minbft.NewReplica(cfg, replicaStacks[i], logger)
		replicas = append(replicas, replica)
	}

	// clients
	for i := 0; i < numClient; i++ {
		au, _ := authen.New([]api.AuthenticationRole{api.ClientAuthen}, testClientID, bytes.NewBuffer(testKeys))

		clientStacks = append(clientStacks, &testClientStack{clientConnectors[i], au})

		client, _ := cl.New(testClientID, cfg.N(), cfg.F(), clientStacks[i])
		clients = append(clients, client)
	}

	connectReplicas(replicaConnectors, replicas)
	connectClients(clientConnectors, replicas)
}

func createReplicaConnectors(numReplica int, n int) []*dummyConnector.ReplicaConnector {
	connectors := make([]*dummyConnector.ReplicaConnector, n)
	for i := range connectors {
		connectors[i] = dummyConnector.New(numReplica)
	}
	return connectors
}

func connectReplicas(connectors []*dummyConnector.ReplicaConnector, replicas []*minbft.Replica) {
	for i, connector := range connectors {
		peers := makeReplicaMap(replicas)
		delete(peers, uint32(i)) // avoid connecting replica to itself
		connector.ConnectManyReplicas(peers)
	}
}

func connectClients(connectors []*dummyConnector.ReplicaConnector, replicas []*minbft.Replica) {
	peers := makeReplicaMap(replicas)
	for _, connector := range connectors {
		connector.ConnectManyReplicas(peers)
	}
}

func makeReplicaMap(replicas []*minbft.Replica) map[uint32]api.MessageStreamHandler {
	replicaMap := make(map[uint32]api.MessageStreamHandler)
	for i, r := range replicas {
		replicaMap[uint32(i)] = r
	}
	return replicaMap
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
