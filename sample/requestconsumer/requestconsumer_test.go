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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testMessage = []byte("accepted message")
)

func mockBlock(prevBlock *SimpleBlock, payload []byte) *SimpleBlock {
	var height uint64
	var prevBlockHash []byte

	if prevBlock != nil {
		height = prevBlock.Height + 1
		prevBlockHash = prevBlock.Hash()
	} else {
		height = uint64(1)
	}

	return &SimpleBlock{height, prevBlockHash, payload}
}

func mockResult(block *SimpleBlock) []byte {
	res, err := json.Marshal(block)
	if err != nil {
		panic(err)
	}
	return res
}

func testSimpleBlockMarshaler(t *testing.T) {
	testHash := blockHashAlgo.New().Sum([]byte("test"))
	block := &SimpleBlock{
		Height:  20,
		Payload: make([]byte, 30),
	}
	copy(block.PrevBlockHash[:], testHash)

	blockBytes, err := block.MarshalBinary()
	assert.NoError(t, err)

	unmarshaledBlock := &SimpleBlock{}
	err = unmarshaledBlock.UnmarshalBinary(blockBytes)
	assert.NoError(t, err)
	assert.Equal(t, block, unmarshaledBlock)
}

func testSimpleLedger(t *testing.T) {
	l := NewSimpleLedger()

	assert.Equal(t, uint64(0), l.length, "wrong initial length")
	assert.Nil(t, l.StateDigest(), "state digest of empty ledger is not nil")

	// deliver n accepted messages
	n := 5
	block := mockBlock(nil, testMessage)
	for i := 0; i < n; i++ {
		resChan := l.Deliver(testMessage)
		assert.Equal(t, mockResult(block), <-resChan)
		block = mockBlock(block, testMessage)
	}

	assert.Equal(t, uint64(n), l.length)
	assert.Equal(t, n, len(l.blocks))
	for i := 0; i < n; i++ {
		assert.Equal(t, uint64(i+1), l.blocks[i].Height)
	}
}

func TestRequestConsumer(t *testing.T) {
	t.Run("SimpleBlockMarshaler", testSimpleBlockMarshaler)
	t.Run("SimpleLedger", testSimpleLedger)
}
