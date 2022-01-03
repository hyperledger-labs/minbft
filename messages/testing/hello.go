// Copyright (c) 2020 NEC Laboratories Europe GmbH.
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

func DoTestHello(t *testing.T, impl messages.MessageImpl) {
	t.Run("Fields", func(t *testing.T) {
		r := rand.Uint32()
		h := impl.NewHello(r)
		require.Equal(t, r, h.ReplicaID())
	})
	t.Run("Marshaling", func(t *testing.T) {
		h := RandHello(impl)
		RequireHelloEqual(t, h, RemarshalMsg(impl, h).(messages.Hello))
	})
}

func RandHello(impl messages.MessageImpl) messages.Hello {
	return impl.NewHello(rand.Uint32())
}

func RequireHelloEqual(t *testing.T, h1, h2 messages.Hello) {
	require.Equal(t, h1.ReplicaID(), h2.ReplicaID())
}
