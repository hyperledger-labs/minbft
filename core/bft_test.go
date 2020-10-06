// Copyright (c) 2020 NEC Solution Innovators, Ltd.
//
// Authors: Naoya Horiguchi <naoya.horiguchi@nec.com>
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
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/hyperledger-labs/minbft/api"
	cl "github.com/hyperledger-labs/minbft/client"
	minbft "github.com/hyperledger-labs/minbft/core"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
	usigui "github.com/hyperledger-labs/minbft/core/internal/usig-ui"
	"github.com/hyperledger-labs/minbft/core/internal/utils"
	"github.com/hyperledger-labs/minbft/messages"
	protobufMessages "github.com/hyperledger-labs/minbft/messages/protobuf"
	authen "github.com/hyperledger-labs/minbft/sample/authentication"
	"github.com/hyperledger-labs/minbft/sample/config"
	"github.com/hyperledger-labs/minbft/sample/conn/common/replicastub"
	"github.com/hyperledger-labs/minbft/sample/requestconsumer"
	"github.com/hyperledger-labs/minbft/usig"
	logging "github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
)

const (
	stepInterval        = 20 * time.Millisecond
	sendMessageInterval = 10 * time.Millisecond
)

type testReplicaStackBft interface {
	api.ReplicaConnector
	api.Authenticator
	api.RequestConsumer

	BlockHeight() uint64
}

type testReplicaStackNormal struct {
	api.ReplicaConnector
	api.Authenticator
	*requestconsumer.SimpleLedger
}

func (s *testReplicaStackNormal) BlockHeight() uint64 {
	return s.SimpleLedger.GetLength()
}

type testReplicaStackSimulated struct {
	api.ReplicaConnector
	api.Authenticator
	*requestconsumer.SimpleLedger
}

func (s *testReplicaStackSimulated) BlockHeight() uint64 {
	return s.SimpleLedger.GetLength()
}

type testClientStackBft struct {
	api.ReplicaConnector
	api.Authenticator
}

var (
	replicaStacksBft []testReplicaStackBft
	clientStacksBft  []*testClientStackBft

	// mapping (client ID, seq) to actual request messages.
	requests map[uint32]map[uint64]messages.Request

	// mapping "prepare ID" to actual prepare messages.
	// This mapping is used only by simulators to refer to
	// the simulated prepare messages in commit phase.
	// It's accessed only from main thread, so no need of locking.
	prepares map[uint32]messages.Prepare

	// mapping (client ID, replica ID) to message channel
	// for reply messages.
	replyChans map[uint32]map[uint32]chan (messages.Message)

	// The request sequence number changes in each testing
	// because it's determined by the time of the first
	// request message from a client. Simulator keeps the
	// first sequence numbers from each client to define expected
	// seq numbers with relative values from the base values.
	baseRequestSequence map[uint32]uint64

	// Simulator keeps all received messages from the target
	// replica to check that scenarios works as expected.
	receivedMessages []receivedMessage

	// Some of the above data structures are shared between goroutines
	// running for simulated nodes, so we need mutual exclusion.
	// These locks could be more fine-grained, but the single lock
	// should be OK for simple scenarios with a few nodes.
	requestLock             sync.Mutex
	replyLock               sync.Mutex
	baseRequestSequenceLock sync.Mutex
	receivedMessageLock     sync.Mutex

	// mapping (sender replica ID, receiver replica ID) to
	// MessageLog borrowed by the Replica instances.
	unicastMessage map[uint32](map[uint32]messagelog.MessageLog)

	// Simulator has full access to any authentication components
	// of all simlated replicas and clients.
	uiAssigner    map[uint32]usigui.UIAssigner
	messageSigner map[uint32]utils.MessageSigner
	auths         map[uint32]api.Authenticator
	cliSigner     map[uint32]api.Authenticator
)

var messageImpl = protobufMessages.NewImpl()

func resetFixtureBft() {
	replicas = nil
	replicaStacksBft = nil
	clients = nil
	clientStacksBft = nil
	replicaStubs = nil

	requests = make(map[uint32]map[uint64]messages.Request)
	prepares = make(map[uint32]messages.Prepare)

	replyChans = make(map[uint32]map[uint32](chan messages.Message))
	baseRequestSequence = make(map[uint32]uint64)
	receivedMessages = nil
	unicastMessage = make(map[uint32](map[uint32]messagelog.MessageLog))

	uiAssigner = make(map[uint32]usigui.UIAssigner)
	messageSigner = make(map[uint32]utils.MessageSigner)
	auths = make(map[uint32]api.Authenticator)
	cliSigner = make(map[uint32]api.Authenticator)
}

type receivedMessage struct {
	// receiver's ID could be replica ID for PREPARE/COMMIT or client ID for REPLY.
	receiver uint32
	Msg      messages.Message
}

// Implements NonDefaultMessageHandler on testReplicaStackSimulated() to use
// customized message handlers for testing.
func (t *testReplicaStackSimulated) GenerateMessageHandlers(id, n uint32, messageLog messagelog.MessageLog, unicastLogs map[uint32]messagelog.MessageLog, configer api.Configer, stack minbft.Stack, logger *logging.Logger) (handleOwnMessage, handlePeerMessage, handleClientMessage, handleHelloMessage minbft.MessageHandler) {
	unicastMessage[id] = unicastLogs
	uiAssigner[id] = usigui.MakeUIAssigner(stack, messages.AuthenBytes)
	messageSigner[id] = utils.MakeMessageSigner(stack, messages.AuthenBytes)

	// Simulator does not care about handling its own generated messages.
	handleOwnMessage = func(msg messages.Message) (_ <-chan messages.Message, new bool, err error) {
		return nil, new, nil
	}

	// Messages received by peers in simulator will be
	// forwarded to the handler of the simulator.
	handlePeerMessage = func(msg messages.Message) (_ <-chan messages.Message, new bool, err error) {
		receivedMessageLock.Lock()
		receivedMessages = append(receivedMessages, receivedMessage{id, msg})
		receivedMessageLock.Unlock()
		return nil, new, nil
	}

	handleClientMessage = func(msg messages.Message) (_ <-chan messages.Message, new bool, err error) {
		switch m := msg.(type) {
		case messages.Request:
			clientID := m.ClientID()
			seq := m.Sequence()

			// keep the first sequence number of each client
			baseRequestSequenceLock.Lock()
			if _, ok := baseRequestSequence[clientID]; !ok {
				baseRequestSequence[clientID] = seq
			}
			relativeSequence := seq - baseRequestSequence[clientID]
			baseRequestSequenceLock.Unlock()

			requestLock.Lock()
			requests[clientID][relativeSequence] = m
			requestLock.Unlock()
			replyChan := make(chan messages.Message, 1)
			go func() {
				defer close(replyChan)
				replyLock.Lock()
				tmpReplyChan := replyChans[clientID][id]
				replyLock.Unlock()
				for m2 := range tmpReplyChan {
					// Note that sender's ID in REPLY message should be
					// the sender of the (f+1)-th REPLY message.
					receivedMessageLock.Lock()
					receivedMessages = append(receivedMessages, receivedMessage{clientID, m2})
					receivedMessageLock.Unlock()
					replyChan <- m2
					// Assuming that only one REQUEST from the same client
					// can be handled at one time.
					break
				}
			}()
			return replyChan, new, nil
		}
		return nil, new, nil
	}

	handleHelloMessage = minbft.MakeHelloHandler(id, n, messageLog, unicastLogs)
	return
}

// The replica with replica ID @targetReplica behaves as the test target replica,
// and all other replicas and all clients are the part of the simulator.
func makeEnvironmentNetwork(numReplica, numClient, targetReplica int, testKeys []byte, cfg api.Configer) {
	for i := 0; i < numReplica; i++ {
		replicaStubs = append(replicaStubs, replicastub.New())
	}

	for i := 0; i < numClient; i++ {
		cid := uint32(i)
		au, _ := authen.New([]api.AuthenticationRole{api.ClientAuthen}, cid, bytes.NewBuffer(testKeys))
		cliSigner[cid] = au
		conn := newClientSideConnector()
		stack := &testClientStackBft{conn, au}
		clientStacksBft = append(clientStacksBft, stack)

		client, _ := cl.New(cid, cfg.N(), cfg.F(), stack)
		clients = append(clients, client)
		replyChans[cid] = make(map[uint32]chan messages.Message)
		requests[cid] = make(map[uint64]messages.Request)
	}

	for i := 0; i < numReplica; i++ {
		id := uint32(i)
		sigAuth, _ := authen.NewWithSGXUSIG([]api.AuthenticationRole{api.ReplicaAuthen, api.USIGAuthen}, id, bytes.NewBuffer(testKeys), usigEnclaveFile)
		auths[id] = sigAuth
		ledger := requestconsumer.NewSimpleLedger()
		conn := newReplicaSideConnector(id)
		if i == targetReplica {
			stack := &testReplicaStackNormal{conn, sigAuth, ledger}
			replicaStacksBft = append(replicaStacksBft, stack)

			replica, _ := minbft.New(id, cfg, stack)
			replicas = append(replicas, replica)
			replicaStubs[i].AssignReplica(replica)
		} else { // simulated replicas
			stack := &testReplicaStackSimulated{conn, sigAuth, ledger}
			replicaStacksBft = append(replicaStacksBft, stack)

			replica, _ := minbft.New(id, cfg, stack)
			replicas = append(replicas, replica)
			replicaStubs[i].AssignReplica(replica)
		}

		for j := 0; j < numClient; j++ {
			replyChans[uint32(j)][uint32(i)] = make(chan messages.Message)
		}
	}

	// to separate logs between connecting logic and test logic
	time.Sleep(10 * time.Millisecond)
}

type scenario struct {
	Steps []step `yaml:"steps,omitempty"`
}

type step struct {
	Type     string              `yaml:"type"`
	Messages []map[string]string `yaml:"messages,omitempty"`
}

func getMsgAttr32(msg map[string]string, elm string) uint32 {
	var tmp uint64
	var err error

	tmp, err = strconv.ParseUint(msg[elm], 10, 32)
	if err != nil {

		panic(fmt.Sprintf("parse '%v': %s", elm, err))
	}
	return uint32(tmp)
}

func getMsgAttr64(msg map[string]string, elm string) uint64 {
	var tmp uint64
	var err error

	tmp, err = strconv.ParseUint(msg[elm], 10, 64)
	if err != nil {
		panic(fmt.Sprintf("parse '%v': %s", elm, err))
	}
	return tmp
}

func sendRequest(msg map[string]string) {
	sender := getMsgAttr32(msg, "sender")

	go func() {
		clientobj := clients[sender]
		<-clientobj.Request([]byte(msg["operation"]))
	}()
}

func sendPrepare(msg map[string]string) {
	client := getMsgAttr32(msg, "client")
	sender := getMsgAttr32(msg, "sender")
	receiver := getMsgAttr32(msg, "receiver")
	view := getMsgAttr64(msg, "view")
	seq := getMsgAttr64(msg, "seq")
	prepareid := getMsgAttr32(msg, "prepareid")
	cv := uint64(0)
	if _, ok := msg["cv"]; ok {
		cv = getMsgAttr64(msg, "cv")
	}
	duplicates := uint64(0)
	if _, ok := msg["duplicates"]; ok {
		duplicates = getMsgAttr64(msg, "duplicates")
	}

	var request struct {
		Operation string
	}
	if err := yaml.UnmarshalStrict([]byte(msg["request"]), &request); err != nil {
		fmt.Printf("failed to parse request %s\n", err)
		return
	}

	requestLock.Lock()
	req := requests[client][seq]
	requestLock.Unlock()
	if req == nil {
		fmt.Printf("Failed to retrieve REQUEST from cache for clientID:%d, relative sequence:%d\n", client, seq)
		return
	}
	baseRequestSequenceLock.Lock()
	seq += baseRequestSequence[client]
	baseRequestSequenceLock.Unlock()

	if request.Operation != "" {
		fmt.Printf("replace operation in embedded request with '%s'\n", request.Operation)
		req = messageImpl.NewRequest(client, seq, []byte(request.Operation))
		sig, _ := cliSigner[client].GenerateMessageAuthenTag(api.ClientAuthen, messages.AuthenBytes(req))
		req.SetSignature(sig)
	}

	prepare := messageImpl.NewPrepare(sender, view, req)
	if cv > 0 {
		authenBytes := messages.AuthenBytes(prepare)
		uiBytes, _ := auths[sender].GenerateMessageAuthenTag(api.USIGAuthen, authenBytes)
		// setting arbitrary value of CV to the prepare message
		ui := &usig.UI{Counter: cv, Cert: uiBytes[8:]}
		prepare.SetUI(ui)
	} else {
		uiAssigner[sender](prepare)
	}

	for i := 0; i < int(duplicates)+1; i++ {
		unicastMessage[sender][receiver].Append(prepare)
	}
	prepares[prepareid] = prepare
}

func sendCommit(msg map[string]string) {
	sender := getMsgAttr32(msg, "sender")
	receiver := getMsgAttr32(msg, "receiver")
	prepareid := getMsgAttr32(msg, "prepareid")
	prepare := prepares[prepareid]
	cv := uint64(0)
	if _, ok := msg["cv"]; ok {
		cv = getMsgAttr64(msg, "cv")
	}
	duplicates := uint64(0)
	if _, ok := msg["duplicates"]; ok {
		duplicates = getMsgAttr64(msg, "duplicates")
	}

	commit := messageImpl.NewCommit(sender, prepare)
	if cv > 0 {
		authenBytes := messages.AuthenBytes(commit)
		uiBytes, _ := auths[sender].GenerateMessageAuthenTag(api.USIGAuthen, authenBytes)
		ui := &usig.UI{Counter: cv, Cert: uiBytes[8:]}
		commit.SetUI(ui)
	} else {
		uiAssigner[sender](commit)
	}

	for i := 0; i < int(duplicates)+1; i++ {
		unicastMessage[sender][receiver].Append(commit)
	}
}

func sendReply(msg map[string]string) {
	sender := getMsgAttr32(msg, "sender")
	receiver := getMsgAttr32(msg, "receiver")
	baseRequestSequenceLock.Lock()
	seq := getMsgAttr64(msg, "seq") + baseRequestSequence[receiver]
	baseRequestSequenceLock.Unlock()
	reply := messageImpl.NewReply(sender, receiver, seq, []byte(msg["result"]))
	messageSigner[sender](reply)
	replyLock.Lock()
	replyChans[receiver][sender] <- reply
	replyLock.Unlock()
	// replyChan in handleClientMessage() should be closed after handling REPLY.
}

func runStepSend(step step) {
	for _, msg := range step.Messages {
		if msg["type"] == "request" {
			sendRequest(msg)
		} else if msg["type"] == "prepare" {
			sendPrepare(msg)
		} else if msg["type"] == "commit" {
			sendCommit(msg)
		} else if msg["type"] == "reply" {
			sendReply(msg)
		}
		time.Sleep(sendMessageInterval)
	}
}

func runStepReceive(step step) {
	for _, msg := range step.Messages {
		prepareid := getMsgAttr32(msg, "prepareid")
		for _, rmsg := range receivedMessages {
			switch msg2 := rmsg.Msg.(type) {
			case messages.Prepare:
				prepares[prepareid] = msg2
			}
		}
	}
}

// func checkWarning(

func runStepCheck(step step) {
	nr_msgs := len(step.Messages)
	nr_received_msgs := len(receivedMessages)

	fmt.Printf("====> confirming messages from target replica\n")
	fmt.Printf("%d messages received, %d expected\n", nr_received_msgs, nr_msgs)

	for i, msg := range receivedMessages {
		faillabel := fmt.Sprintf("FAIL (%d/%d)", i+1, nr_received_msgs)

		if i >= nr_msgs {
			fmt.Printf("%s: received unexpected message %+v\n", faillabel, messages.Stringify(msg.Msg))
			continue
		}

		emsg := step.Messages[i]
		switch msg2 := msg.Msg.(type) {
		case messages.Prepare:
			if emsg["type"] != "prepare" {
				fmt.Printf("%s: message type is prepare, expected %s\n", faillabel, emsg["type"])
				continue
			}
			id := getMsgAttr32(emsg, "receiver")
			senderid := getMsgAttr32(emsg, "replica")
			view := getMsgAttr64(emsg, "view")
			cv := getMsgAttr64(emsg, "cv")
			if msg2.ReplicaID() != senderid {
				fmt.Printf("%s: msg from %d, expected %d\n", faillabel, msg2.ReplicaID(), senderid)
			}
			if msg.receiver != id {
				fmt.Printf("%s: msg to %d, expected %d\n", faillabel, msg.receiver, id)
			}
			if msg2.View() != view {
				fmt.Printf("%s: msg view %d, expected %d\n", faillabel, msg2.View(), view)
			}
			if msg2.UI().Counter != cv {
				fmt.Printf("%s: msg CV %d, expected %d\n", faillabel, msg2.UI().Counter, cv)
			}
		case messages.Commit:
			if emsg["type"] != "commit" {
				fmt.Printf("%s: message type is commit, expected %s\n", faillabel, emsg["type"])
				continue
			}
			sender := getMsgAttr32(emsg, "sender")
			receiver := getMsgAttr32(emsg, "receiver")
			cv := getMsgAttr64(emsg, "cv")
			if msg2.ReplicaID() != sender {
				fmt.Printf("%s: msg from %d, expected %d\n", faillabel, msg2.ReplicaID(), sender)
			}
			if msg.receiver != receiver {
				fmt.Printf("%s: msg to %d, expected %d\n", faillabel, msg.receiver, receiver)
			}
			if msg2.UI().Counter != cv {
				fmt.Printf("%s: msg CV %d, expected %d\n", faillabel, msg2.UI().Counter, cv)
			}
		case messages.Reply:
			if emsg["type"] != "reply" {
				fmt.Printf("%s: message type is reply, expected %s\n", faillabel, emsg["type"])
				continue
			}
			sender := getMsgAttr32(emsg, "sender")
			receiver := getMsgAttr32(emsg, "receiver")
			baseRequestSequenceLock.Lock()
			seq := getMsgAttr64(emsg, "seq") + baseRequestSequence[receiver]
			baseRequestSequenceLock.Unlock()
			if msg2.ReplicaID() != sender {
				fmt.Printf("%s: msg from %d, expected %d\n", faillabel, msg2.ReplicaID(), sender)
			}
			if msg.receiver != receiver {
				fmt.Printf("%s: msg to %d, expected %d\n", faillabel, msg.receiver, receiver)
			}
			if msg2.Sequence() != seq {
				fmt.Printf("%s: msg sequence %d, expected %d\n", faillabel, msg2.Sequence(), seq)
			}
			if string(msg2.Result()) != emsg["result"] {
				fmt.Printf("%s: msg result %s, expected %s\n", faillabel, string(msg2.Result()), emsg["result"])
			}
		}
	}
}

func runSimulator(input string) (ret bool) {
	var scenario scenario
	err := yaml.Unmarshal([]byte(input), &scenario)
	if err != nil {
		panic(err)
	}

	finished := make(chan bool)

	go func() {
		defer close(finished)

		for i, step := range scenario.Steps {
			fmt.Printf("Step %d/%d: %s %d messages\n", i+1, len(scenario.Steps), step.Type, len(step.Messages))
			if step.Type == "send" {
				runStepSend(step)
			} else if step.Type == "receive" {
				runStepReceive(step)
			} else if step.Type == "check" {
				runStepCheck(step)
			}
			time.Sleep(stepInterval)
		}
		finished <- true
	}()

	ret = <-finished
	fmt.Printf("=== Simulator done.\n")
	time.Sleep(10 * time.Millisecond)
	return
}

func initTestnetPeersBft(numReplica int, numClient int, targetReplica int) {
	resetFixtureBft()

	// generate config, keys
	testCfg := createTestCfg(numReplica)
	testKeys := createTestKeys(numReplica, numClient)

	cfg := config.New() // configer shared by all replicas
	err := cfg.ReadConfig(bytes.NewBuffer(testCfg), "yaml")
	if err != nil {
		panic(err)
	}

	makeEnvironmentNetwork(numReplica, numClient, targetReplica, testKeys, cfg)
}

func testSimpleScenarioBackup(t *testing.T) {
	input := `
steps:
  - type: send
    messages:
      - type: request
        sender: 0
        operation: test request message
      - type: prepare
        client: 0
        sender: 0
        receiver: 1
        view: 0
        seq: 0
        prepareid: 0
      - type: commit
        sender: 2
        receiver: 1
        prepareid: 0
      - type: reply
        sender: 0
        receiver: 0
        seq: 0
        result: '{"Height":1,"PrevBlockHash":null,"Payload":"dGVzdCByZXF1ZXN0IG1lc3NhZ2U="}'
  - type: check
    messages:
      - type: commit
        sender: 1
        receiver: 2
        cv: 1
      - type: commit
        sender: 1
        receiver: 0
        cv: 1
      - type: reply
        sender: 0
        receiver: 0
        seq: 0
        result: '{"Height":1,"PrevBlockHash":null,"Payload":"dGVzdCByZXF1ZXN0IG1lc3NhZ2U="}'
`

	result := runSimulator(input)

	assert.True(t, result)
	assert.Equal(t, uint64(1), replicaStacksBft[uint32(1)].BlockHeight())
}

func testSimpleScenarioPrimary(t *testing.T) {
	input := `
steps:
  - type: send
    messages:
      - type: request
        sender: 0
        operation: test request message
  - type: receive
    messages:
      - type: prepare
        replica: 0
        view: 0
        client: 0
        seq: 0
        destination: 1
        prepareid: 0
  - type: send
    messages:
      - type: commit
        sender: 1
        receiver: 0
        prepareid: 0
      - type: commit
        sender: 2
        receiver: 0
        prepareid: 0
      - type: reply
        sender: 1
        receiver: 0
        seq: 0
        result: '{"Height":1,"PrevBlockHash":null,"Payload":"dGVzdCByZXF1ZXN0IG1lc3NhZ2U="}'
  - type: check
    messages:
      - type: prepare
        cv: 1
        replica: 0
        view: 0
        client: 0
        seq: 0
        sender: 1
        receiver: 1
      - type: prepare
        cv: 1
        replica: 0
        view: 0
        client: 0
        seq: 0
        sender: 1
        receiver: 2
      - type: reply
        seq: 0
        result: '{"Height":1,"PrevBlockHash":null,"Payload":"dGVzdCByZXF1ZXN0IG1lc3NhZ2U="}'
        sender: 1
        receiver: 0
`

	result := runSimulator(input)

	assert.True(t, result)
	assert.Equal(t, uint64(1), replicaStacksBft[uint32(0)].BlockHeight())
}

func testTwoClientScenarioBackup(t *testing.T) {
	input := `
steps:
  - type: send
    messages:
      - type: request
        sender: 0
        operation: request from client 0
      - type: request
        sender: 1
        operation: request from client 1
      - type: prepare
        client: 0
        sender: 0
        receiver: 1
        view: 0
        seq: 0
        prepareid: 0
      - type: commit
        sender: 2
        receiver: 1
        prepareid: 0
      - type: prepare
        client: 1
        sender: 0
        receiver: 1
        view: 0
        seq: 0
        prepareid: 1
      - type: commit
        sender: 2
        receiver: 1
        prepareid: 1
      - type: reply
        sender: 0
        receiver: 0
        seq: 0
        result: '{"Height":1,"PrevBlockHash":null,"Payload":"dGVzdCByZXF1ZXN0IG1lc3NhZ2U="}'
      - type: reply
        sender: 0
        receiver: 1
        seq: 0
        result: '{"Height":2,"PrevBlockHash":"6qdy4iR8cHF0jarsHSgtLuu4towvKUNrWllDw2yLarI=","Payload":"cmVxdWVzdCBmcm9tIGNsaWVudCAx"}'
  - type: check
    messages:
      - type: commit
        sender: 1
        receiver: 2
        cv: 1
      - type: commit
        sender: 1
        receiver: 0
        cv: 1
      - type: commit
        sender: 1
        receiver: 2
        cv: 2
      - type: commit
        sender: 1
        receiver: 0
        cv: 2
      - type: reply
        sender: 0
        receiver: 0
        seq: 0
        result: '{"Height":1,"PrevBlockHash":null,"Payload":"dGVzdCByZXF1ZXN0IG1lc3NhZ2U="}'
      - type: reply
        sender: 0
        receiver: 1
        seq: 0
        result: '{"Height":2,"PrevBlockHash":"6qdy4iR8cHF0jarsHSgtLuu4towvKUNrWllDw2yLarI=","Payload":"cmVxdWVzdCBmcm9tIGNsaWVudCAx"}'
`

	result := runSimulator(input)

	assert.True(t, result)
	assert.Equal(t, uint64(2), replicaStacksBft[uint32(1)].BlockHeight())
}

func TestSimulator(t *testing.T) {
	testCases := []struct {
		numReplica    int
		numClient     int
		targetReplica int
		testcase      func(t *testing.T)
	}{
		{numReplica: 3, numClient: 1, targetReplica: 1, testcase: testSimpleScenarioBackup},
		{numReplica: 3, numClient: 1, targetReplica: 0, testcase: testSimpleScenarioPrimary},
		{numReplica: 3, numClient: 2, targetReplica: 1, testcase: testTwoClientScenarioBackup},
	}
	for _, tc := range testCases {
		initTestnetPeersBft(tc.numReplica, tc.numClient, tc.targetReplica)

		testname := runtime.FuncForPC(reflect.ValueOf(tc.testcase).Pointer()).Name()
		testname = filepath.Ext(testname)[1:]
		t.Run(fmt.Sprintf("%s/r=%d/c=%d/t=%d", testname, tc.numReplica, tc.numClient, tc.targetReplica), tc.testcase)
	}
}
