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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/messages"
)

func TestReply(t *testing.T) {
	impl := NewImpl()

	t.Run("Fields", func(t *testing.T) {
		r := rand.Uint32()
		cl := rand.Uint32()
		seq := rand.Uint64()
		res := randBytes()
		reply := impl.NewReply(r, cl, seq, res)
		require.Equal(t, r, reply.ReplicaID())
		require.Equal(t, cl, reply.ClientID())
		require.Equal(t, seq, reply.Sequence())
		require.Equal(t, res, reply.Result())
	})
	t.Run("SignedPayload", func(t *testing.T) {
		reply := randReply(impl)
		r := reply.ReplicaID()
		cl := reply.ClientID()
		seq := reply.Sequence()
		res := reply.Result()
		sp := reply.SignedPayload()
		require.NotEqual(t, sp, newTestReply(impl, r, rand.Uint32(), seq, res).SignedPayload())
		require.NotEqual(t, sp, newTestReply(impl, r, cl, rand.Uint64(), res).SignedPayload())
		require.NotEqual(t, sp, newTestReply(impl, r, cl, seq, randBytes()).SignedPayload())
	})
	t.Run("SetSignature", func(t *testing.T) {
		reply := randReply(impl)
		sig := testSig(reply.SignedPayload())
		reply.SetSignature(sig)
		require.Equal(t, sig, reply.Signature())
	})
	t.Run("Marshaling", func(t *testing.T) {
		reply := randReply(impl)
		requireReplyEqual(t, reply, remarshalMsg(impl, reply).(messages.Reply))
	})
}

func randReply(impl messages.MessageImpl) messages.Reply {
	return newTestReply(impl, rand.Uint32(), rand.Uint32(), rand.Uint64(), randBytes())
}

func newTestReply(impl messages.MessageImpl, r, cl uint32, seq uint64, res []byte) messages.Reply {
	reply := impl.NewReply(r, cl, seq, res)
	reply.SetSignature(testSig(reply.SignedPayload()))
	return reply
}

func requireReplyEqual(t *testing.T, reply1, reply2 messages.Reply) {
	require.Equal(t, reply1.ReplicaID(), reply2.ReplicaID())
	require.Equal(t, reply1.ClientID(), reply2.ClientID())
	require.Equal(t, reply1.Sequence(), reply2.Sequence())
	require.Equal(t, reply1.Result(), reply2.Result())
	require.Equal(t, reply1.SignedPayload(), reply2.SignedPayload())
	require.Equal(t, reply1.Signature(), reply2.Signature())
}
