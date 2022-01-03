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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/messages"
)

func DoTestCommit(t *testing.T, impl messages.MessageImpl) {
	const f = 1
	const n = 3
	const maxView = 1
	const maxNrRequests = 2

	reqs := make([]messages.Request, maxNrRequests)
	for i := range reqs {
		reqs[i] = MakeTestReq(impl, 0, uint64(i), nil)
	}

	for k := len(reqs); k >= 0; k-- {
		reqs := reqs[:k]
		for v := uint64(0); v <= maxView; v++ {
			p := uint32(v % uint64(n))
			i := 0
			for logs := range GenerateMessageLogs(impl, f, n, v, reqs) {
				for j, m := range logs[p] {
					for r := uint32(0); r < n; r++ {
						if r == p {
							continue
						}
						cv := LastLogCV(logs[r])
						t.Run(fmt.Sprintf("NrRequests=%d/View=%d/Log=%d/Proposal=%d/Replica=%d", k, v, i, j, r), func(t *testing.T) {
							testCommit(t, impl, r, m, cv)
						})
					}
				}
				i++
			}
			if testing.Short() {
				return
			}
		}
	}
}

func testCommit(t *testing.T, impl messages.MessageImpl, r uint32, prop messages.CertifiedMessage, cv uint64) {
	t.Run("Fields", func(t *testing.T) {
		comm := impl.NewCommit(r, prop)
		require.Equal(t, r, comm.ReplicaID())
		RequireCertMsgEqual(t, prop, comm.Proposal())
	})
	t.Run("SetUI", func(t *testing.T) {
		comm := impl.NewCommit(r, prop)
		ui := MakeTestUI(cv, messages.AuthenBytes(comm))
		comm.SetUI(ui)
		require.Equal(t, ui, comm.UI())
	})
	t.Run("Marshaling", func(t *testing.T) {
		comm := MakeTestComm(impl, r, prop, cv)
		RequireCommEqual(t, comm, RemarshalMsg(impl, comm).(messages.Commit))
	})
}

func MakeTestComm(impl messages.MessageImpl, r uint32, prop messages.CertifiedMessage, cv uint64) messages.Commit {
	comm := impl.NewCommit(r, prop)
	comm.SetUI(MakeTestUI(cv, messages.AuthenBytes(comm)))
	return comm
}

func RequireCommEqual(t *testing.T, comm1, comm2 messages.Commit) {
	require.Equal(t, comm1.ReplicaID(), comm2.ReplicaID())
	RequireCertMsgEqual(t, comm1.Proposal(), comm2.Proposal())
	require.Equal(t, comm1.UI(), comm2.UI())
}
