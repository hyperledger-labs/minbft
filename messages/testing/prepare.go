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

func DoTestPrepare(t *testing.T, impl messages.MessageImpl) {
	t.Run("Fields", func(t *testing.T) {
		r := rand.Uint32()
		v := rand.Uint64()
		req := RandReq(impl)
		prep := impl.NewPrepare(r, v, req)
		require.Equal(t, r, prep.ReplicaID())
		require.Equal(t, v, prep.View())
		RequireReqEqual(t, req, prep.Request())
	})
	t.Run("SetUI", func(t *testing.T) {
		prep := RandPrep(impl)
		ui := RandUI(messages.AuthenBytes(prep))
		prep.SetUI(ui)
		require.Equal(t, ui, prep.UI())
	})
	t.Run("Marshaling", func(t *testing.T) {
		prep := RandPrep(impl)
		RequirePrepEqual(t, prep, RemarshalMsg(impl, prep).(messages.Prepare))
	})
}

func RandPrep(impl messages.MessageImpl) messages.Prepare {
	return MakeTestPrep(impl, rand.Uint32(), rand.Uint64(), RandReq(impl), rand.Uint64())
}

func MakeTestPrep(impl messages.MessageImpl, r uint32, v uint64, req messages.Request, cv uint64) messages.Prepare {
	prep := impl.NewPrepare(r, v, req)
	prep.SetUI(MakeTestUI(cv, messages.AuthenBytes(prep)))
	return prep
}

func RequirePrepEqual(t *testing.T, prep1, prep2 messages.Prepare) {
	require.Equal(t, prep1.ReplicaID(), prep2.ReplicaID())
	require.Equal(t, prep1.View(), prep2.View())
	RequireReqEqual(t, prep1.Request(), prep2.Request())
	require.Equal(t, prep1.UI(), prep2.UI())
}
