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

func TestRequest(t *testing.T) {
	impl := NewImpl()

	t.Run("Fields", func(t *testing.T) {
		cl := rand.Uint32()
		seq := rand.Uint64()
		op := randBytes()
		req := impl.NewRequest(cl, seq, op)
		require.Equal(t, cl, req.ClientID())
		require.Equal(t, seq, req.Sequence())
		require.Equal(t, op, req.Operation())
	})
	t.Run("SignedPayload", func(t *testing.T) {
		req := randReq(impl)
		cl := req.ClientID()
		seq := req.Sequence()
		op := req.Operation()
		sp := req.SignedPayload()
		require.NotEqual(t, sp, newTestReq(impl, cl, rand.Uint64(), op).SignedPayload())
		require.NotEqual(t, sp, newTestReq(impl, cl, seq, randBytes()).SignedPayload())
	})
	t.Run("SetSignature", func(t *testing.T) {
		req := randReq(impl)
		sig := testSig(req.SignedPayload())
		req.SetSignature(sig)
		require.Equal(t, sig, req.Signature())
	})
	t.Run("Marshaling", func(t *testing.T) {
		req := randReq(impl)
		requireReqEqual(t, req, remarshalMsg(impl, req).(messages.Request))
	})
}

func randReq(impl messages.MessageImpl) messages.Request {
	return newTestReq(impl, rand.Uint32(), rand.Uint64(), randBytes())
}

func newTestReq(impl messages.MessageImpl, cl uint32, seq uint64, op []byte) messages.Request {
	req := impl.NewRequest(cl, seq, op)
	req.SetSignature(testSig(req.SignedPayload()))
	return req
}

func requireReqEqual(t *testing.T, req1, req2 messages.Request) {
	require.Equal(t, req1.ClientID(), req2.ClientID())
	require.Equal(t, req1.Sequence(), req2.Sequence())
	require.Equal(t, req1.Operation(), req2.Operation())
	require.Equal(t, req1.SignedPayload(), req2.SignedPayload())
	require.Equal(t, req1.Signature(), req2.Signature())
}
