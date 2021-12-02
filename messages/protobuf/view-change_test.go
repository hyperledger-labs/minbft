// Copyright (c) 2021 NEC Laboratories Europe GmbH.
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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/messages"
)

func TestViewChange(t *testing.T) {
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
			i := 0
			for logs := range generateMessageLogs(impl, f, n, v-1, reqs) {
				for r := uint32(0); r < n; r++ {
					cv := lastLogCV(logs[r])
					t.Run(fmt.Sprintf("NrRequests=%d/View=%d/Log=%d/Replica=%d", k, v, i, r), func(t *testing.T) {
						testViewChange(t, impl, r, v, logs[r], randVCCert(impl, f, n, v), cv)
					})
				}
				i++
			}
			if testing.Short() {
				return
			}
		}
	}
}

func testViewChange(t *testing.T, impl messages.MessageImpl, r uint32, v uint64, log messages.MessageLog, vcCert messages.ViewChangeCert, cv uint64) {
	t.Run("Fields", func(t *testing.T) {
		vc := impl.NewViewChange(r, v, log, vcCert)
		require.Equal(t, r, vc.ReplicaID())
		require.Equal(t, v, vc.NewView())
		requireMsgLogEqual(t, log, vc.MessageLog())
		requireVCCertEqual(t, vcCert, vc.ViewChangeCert())
	})
	t.Run("SetUI", func(t *testing.T) {
		vc := impl.NewViewChange(r, v, log, vcCert)
		ui := newTestUI(cv, messages.AuthenBytes(vc))
		vc.SetUI(ui)
		require.Equal(t, ui, vc.UI())
	})
	t.Run("Marshaling", func(t *testing.T) {
		vc := newTestVC(impl, r, v, log, vcCert, cv)
		requireVCEqual(t, vc, remarshalMsg(impl, vc).(messages.ViewChange))
	})
}

func newTestVC(impl messages.MessageImpl, r uint32, v uint64, log messages.MessageLog, vcCert messages.ViewChangeCert, cv uint64) messages.ViewChange {
	vc := impl.NewViewChange(r, v, log, vcCert)
	vc.SetUI(newTestUI(cv, messages.AuthenBytes(vc)))
	return vc
}

func randVCCert(impl messages.MessageImpl, f, n uint32, v uint64) messages.ViewChangeCert {
	var cert messages.ViewChangeCert
	for _, r := range rand.Perm(int(n))[:f+1] {
		cert = append(cert, newTestReqViewChange(impl, uint32(r), v))
	}
	return cert
}

func requireVCEqual(t *testing.T, vc1, vc2 messages.ViewChange) {
	require.Equal(t, vc1.ReplicaID(), vc2.ReplicaID())
	require.Equal(t, vc1.NewView(), vc2.NewView())
	requireMsgLogEqual(t, vc1.MessageLog(), vc2.MessageLog())
	requireVCCertEqual(t, vc2.ViewChangeCert(), vc2.ViewChangeCert())
	require.Equal(t, vc1.UI(), vc2.UI())
}

func requireMsgLogEqual(t *testing.T, log1, log2 messages.MessageLog) {
	require.Equal(t, len(log1), len(log2))
	for i, m1 := range log1 {
		requireCertMsgEqual(t, m1, log2[i])
	}
}

func requireVCCertEqual(t *testing.T, c1, c2 messages.ViewChangeCert) {
	require.Equal(t, len(c1), len(c2))
	for i, rvc1 := range c1 {
		requireReqViewChangeEqual(t, rvc1, c2[i])
	}
}
