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

func TestPrepare(t *testing.T) {
	impl := NewImpl()

	t.Run("Fields", func(t *testing.T) {
		r := rand.Uint32()
		v := rand.Uint64()
		req := randReq(impl)
		prep := impl.NewPrepare(r, v, req)
		require.Equal(t, r, prep.ReplicaID())
		require.Equal(t, v, prep.View())
		requireReqEqual(t, req, prep.Request())
	})
	t.Run("CertifiedPayload", func(t *testing.T) {
		prep := randPrep(impl)
		r := prep.ReplicaID()
		v := prep.View()
		req := prep.Request()
		cp := prep.CertifiedPayload()
		require.NotEqual(t, cp, newTestPrep(impl, r, rand.Uint64(), req).CertifiedPayload())
		require.NotEqual(t, cp, newTestPrep(impl, r, v, randReq(impl)).CertifiedPayload())
	})
	t.Run("SetUIBytes", func(t *testing.T) {
		prep := randPrep(impl)
		uiBytes := newTestUI(prep.CertifiedPayload())
		prep.SetUIBytes(uiBytes)
		require.Equal(t, uiBytes, prep.UIBytes())
	})
	t.Run("Marshaling", func(t *testing.T) {
		prep := randPrep(impl)
		requirePrepEqual(t, prep, remarshalMsg(impl, prep).(messages.Prepare))
	})
}

func randPrep(impl messages.MessageImpl) messages.Prepare {
	return newTestPrep(impl, rand.Uint32(), rand.Uint64(), randReq(impl))
}

func newTestPrep(impl messages.MessageImpl, r uint32, v uint64, req messages.Request) messages.Prepare {
	prep := impl.NewPrepare(r, v, req)
	uiBytes := newTestUI(prep.CertifiedPayload())
	prep.SetUIBytes(uiBytes)
	return prep
}

func requirePrepEqual(t *testing.T, prep1, prep2 messages.Prepare) {
	require.Equal(t, prep1.ReplicaID(), prep2.ReplicaID())
	require.Equal(t, prep1.View(), prep2.View())
	requireReqEqual(t, prep1.Request(), prep2.Request())
	require.Equal(t, prep1.CertifiedPayload(), prep2.CertifiedPayload())
	require.Equal(t, prep1.UIBytes(), prep2.UIBytes())
}
