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

package requestconsumer

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
)

const (
	blockHashAlgo = crypto.SHA256
	hashSize      = sha256.Size
)

//SimpleBlock defines a basic block structure composed of blockheight and previous
//block hash.
type SimpleBlock struct {
	Height        uint64
	PrevBlockHash []byte
	Payload       []byte
}

// MarshalBinary implements the BinaryMarshaler interface for SimpleBlock
func (b *SimpleBlock) MarshalBinary() ([]byte, error) {
	blockBytesBuf := new(bytes.Buffer)
	if err := binary.Write(blockBytesBuf, binary.BigEndian, b.Height); err != nil {
		return nil, fmt.Errorf("SimpleBlock binary.Write failed with b.height: %v", err)
	}
	var prevBlockHash [hashSize]byte
	copy(prevBlockHash[:], b.PrevBlockHash)
	if n, err := blockBytesBuf.Write(prevBlockHash[:]); err != nil || n != hashSize {
		return nil, fmt.Errorf("SimpleBlock binary.Write failed with b.prevBlockHash: %v", err)
	}
	if n, err := blockBytesBuf.Write(b.Payload); err != nil || n != len(b.Payload) {
		return nil, fmt.Errorf("SimpleBlock binary.Write failed with b.payload: %v", err)
	}
	return blockBytesBuf.Bytes(), nil
}

// UnmarshalBinary implements the BinaryUnmarshaler interface for SimpleBlock
func (b *SimpleBlock) UnmarshalBinary(marshaled []byte) error {
	var (
		err error
		n   int
	)
	r := bytes.NewReader(marshaled)
	if err = binary.Read(r, binary.BigEndian, &b.Height); err != nil {
		return fmt.Errorf("SimpleBlock binary.Read failed in b.height: %v", err)
	}
	var prevBlockHash [hashSize]byte
	if n, err = r.Read(prevBlockHash[:]); err != nil || n != hashSize {
		return fmt.Errorf("SimpleBlock binary.Read failed in b.prevBlockHash: %v", err)
	}
	copy(b.PrevBlockHash, prevBlockHash[:])
	if b.Payload, err = ioutil.ReadAll(r); err != nil {
		return fmt.Errorf("SimpleBlock binary.Read failed in b.payload: %v", err)
	}
	return nil
}

// Hash returns block hash
func (b *SimpleBlock) Hash() []byte {
	blockBytes, err := b.MarshalBinary()
	if err != nil {
		panic(err)
	}

	d := blockHashAlgo.New()
	if _, err := d.Write(blockBytes); err != nil {
		panic(err)
	}

	return d.Sum(nil)
}

type acceptedMessage struct {
	requestPayload []byte
	resultChan     chan<- []byte
}

//SimpleLedger implements `RequestConsumer` interface. It defines a queue of delivered
//messages as the `blockchain` and simply print out the new message.
type SimpleLedger struct {
	sync.RWMutex
	acceptedMsgQueue chan *acceptedMessage
	blocks           []*SimpleBlock
	length           uint64
}

//NewSimpleLedger initializes an object of SimpleLedger
func NewSimpleLedger() *SimpleLedger {
	l := &SimpleLedger{
		acceptedMsgQueue: make(chan *acceptedMessage),
	}

	go func() {
		for msg := range l.acceptedMsgQueue {
			// simpleledger: one transaction makes one block
			block := l.appendBlock(msg.requestPayload)

			log.Printf("Received block[%d]: %v", block.Height, string(msg.requestPayload))
			// trivial implementation of the transaction execution and return the result
			blockJSON, err := json.Marshal(block)
			if err != nil {
				panic(err)
			}
			msg.resultChan <- blockJSON
		}
	}()

	return l
}

// GetLength returns the number of the blocks in the ledger
func (l *SimpleLedger) GetLength() uint64 {
	l.RLock()
	defer l.RUnlock()

	return l.length
}

// Deliver implements the RequestConsumer interface. It forwards the committed
// transaction message to the processing queue and returns the result
func (l *SimpleLedger) Deliver(msg []byte) <-chan []byte {
	resultChan := make(chan []byte)
	l.acceptedMsgQueue <- &acceptedMessage{requestPayload: msg, resultChan: resultChan}

	return resultChan
}

// StateDigest returns the hash of the latest block as the digest of the system state
func (l *SimpleLedger) StateDigest() []byte {
	l.RLock()
	defer l.RUnlock()

	if l.length == 0 {
		return nil
	}
	lastBlockBytes, err := l.blocks[l.length-1].MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal block: %v", err))
	}
	return blockHashAlgo.New().Sum(lastBlockBytes)
}

func (l *SimpleLedger) appendBlock(payload []byte) *SimpleBlock {
	l.Lock()
	defer l.Unlock()

	var prevBlockHash []byte
	if l.length > 0 {
		prevBlockHash = l.blocks[l.length-1].Hash()
	}

	l.length++

	block := &SimpleBlock{
		Payload:       payload,
		PrevBlockHash: prevBlockHash,
		Height:        l.length,
	}
	l.blocks = append(l.blocks, block)

	return block
}
