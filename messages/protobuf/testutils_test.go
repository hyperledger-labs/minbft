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
	"fmt"
	"hash/crc32"
	"math/rand"
	"testing"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
	"github.com/stretchr/testify/require"
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

func newTestUI(cv uint64, data []byte) *usig.UI {
	return &usig.UI{
		Counter: cv,
		Cert:    testSig([]byte(fmt.Sprintf("%d:%x", cv, data))),
	}
}

func randUI(data []byte) *usig.UI {
	return newTestUI(rand.Uint64(), data)
}

func requireCertMsgEqual(t *testing.T, m1, m2 messages.CertifiedMessage) {
	switch m1 := m1.(type) {
	case messages.Prepare:
		m2, ok := m2.(messages.Prepare)
		require.True(t, ok)
		requirePrepEqual(t, m1, m2)
	case messages.Commit:
		m2, ok := m2.(messages.Commit)
		require.True(t, ok)
		requireCommEqual(t, m1, m2)
	case messages.ViewChange:
		m2, ok := m2.(messages.ViewChange)
		require.True(t, ok)
		requireVCEqual(t, m1, m2)
	}
}
