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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/messages"
)

func TestCommit(t *testing.T) {
	const f = 1
	const n = 3
	const maxNrViews = 3
	const maxNrRequests = 2

	impl := NewImpl()

	reqs := make([]messages.Request, maxNrRequests)
	for i := range reqs {
		reqs[i] = newTestReq(impl, 0, uint64(i), nil)
	}

	for k := len(reqs); k >= 0; k-- {
		reqs := reqs[:k]
		for v := uint64(1); v < maxNrViews; v++ {
			p := uint32(v % uint64(n))
			i := 0
			for logs := range generateMessageLogs(impl, f, n, v, reqs) {
				for j, m := range logs[p] {
					for r := uint32(0); r < n; r++ {
						if r == p {
							continue
						}
						cv := lastLogCV(logs[r])
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
	// TODO: Test against different new-view certificates
	nv := newTestNV(impl, 1, 1, newTestNVCert(impl), 2)
	testCommit(t, impl, 2, nv, 2)

	prep := newTestPrep(impl, 1, 1, randReq(impl), 3)
	testCommit(t, impl, 1, prep, 2)
}

func testCommit(t *testing.T, impl messages.MessageImpl, r uint32, prop messages.CertifiedMessage, cv uint64) {
	t.Run("Fields", func(t *testing.T) {
		comm := impl.NewCommit(r, prop)
		require.Equal(t, r, comm.ReplicaID())
		requireCertMsgEqual(t, prop, comm.Proposal())
	})
	t.Run("SetUI", func(t *testing.T) {
		comm := impl.NewCommit(r, prop)
		ui := newTestUI(cv, messages.AuthenBytes(comm))
		comm.SetUI(ui)
		require.Equal(t, ui, comm.UI())
	})
	t.Run("Marshaling", func(t *testing.T) {
		comm := newTestComm(impl, r, prop, cv)
		requireCommEqual(t, comm, remarshalMsg(impl, comm).(messages.Commit))
	})
}

func newTestComm(impl messages.MessageImpl, r uint32, prop messages.CertifiedMessage, cv uint64) messages.Commit {
	comm := impl.NewCommit(r, prop)
	comm.SetUI(newTestUI(cv, messages.AuthenBytes(comm)))
	return comm
}

func requireCommEqual(t *testing.T, comm1, comm2 messages.Commit) {
	require.Equal(t, comm1.ReplicaID(), comm2.ReplicaID())
	requireCertMsgEqual(t, comm1.Proposal(), comm2.Proposal())
	require.Equal(t, comm1.UI(), comm2.UI())
}
