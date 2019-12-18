// Copyright (c) 2019 NEC Laboratories Europe GmbH.
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

package protobuf

import (
	"hash/crc32"
	"math/rand"

	"github.com/hyperledger-labs/minbft/messages"
)

func randBytes() []byte {
	return []byte{byte(rand.Int())} // nolint:gosec
}

func testSig(data []byte) []byte {
	cs := crc32.NewIEEE()
	_, _ = cs.Write(data)
	return cs.Sum(nil)
}

func remarshalMsg(impl messages.MessageImpl, msg messages.Message) messages.Message {
	msgBytes, err := msg.MarshalBinary()
	if err != nil {
		panic(err)
	}
	msg2, err := impl.NewFromBinary(msgBytes)
	if err != nil {
		panic(err)
	}
	return msg2
}
