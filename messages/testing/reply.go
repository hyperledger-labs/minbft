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

package testing

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/messages"
)

func DoTestReply(t *testing.T, impl messages.MessageImpl) {
	t.Run("Fields", func(t *testing.T) {
		r := rand.Uint32()
		cl := rand.Uint32()
		seq := rand.Uint64()
		res := RandBytes()
		reply := impl.NewReply(r, cl, seq, res)
		require.Equal(t, r, reply.ReplicaID())
		require.Equal(t, cl, reply.ClientID())
		require.Equal(t, seq, reply.Sequence())
		require.Equal(t, res, reply.Result())
	})
	t.Run("SetSignature", func(t *testing.T) {
		reply := RandReply(impl)
		sig := MakeTestSig(messages.AuthenBytes(reply))
		reply.SetSignature(sig)
		require.Equal(t, sig, reply.Signature())
	})
	t.Run("Marshaling", func(t *testing.T) {
		reply := RandReply(impl)
		RequireReplyEqual(t, reply, RemarshalMsg(impl, reply).(messages.Reply))
	})
}

func RandReply(impl messages.MessageImpl) messages.Reply {
	return MakeTestReply(impl, rand.Uint32(), rand.Uint32(), rand.Uint64(), RandBytes())
}

func MakeTestReply(impl messages.MessageImpl, r, cl uint32, seq uint64, res []byte) messages.Reply {
	reply := impl.NewReply(r, cl, seq, res)
	reply.SetSignature(MakeTestSig(messages.AuthenBytes(reply)))
	return reply
}

func RequireReplyEqual(t *testing.T, reply1, reply2 messages.Reply) {
	require.Equal(t, reply1.ReplicaID(), reply2.ReplicaID())
	require.Equal(t, reply1.ClientID(), reply2.ClientID())
	require.Equal(t, reply1.Sequence(), reply2.Sequence())
	require.Equal(t, reply1.Result(), reply2.Result())
	require.Equal(t, reply1.Signature(), reply2.Signature())
}
