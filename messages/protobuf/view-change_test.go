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
	const maxView = 3
	const maxRequests = 2

	impl := NewImpl()

	preps := make([]messages.Prepare, maxRequests)
	for i := range preps {
		c := uint64(i) + 1
		preps[i] = newTestPrep(impl, 0, 0, randReq(impl), c)
	}

	var logs [][][]messages.MessageLog // r -> v -> i -> log
	for r := uint32(0); r < n; r++ {
		logs = append(logs, [][]messages.MessageLog{})
		for v := uint64(0); v <= maxView; v++ {
			logs[r] = append(logs[r], []messages.MessageLog{})
			if v == 0 {
				var log messages.MessageLog
				for i, prep := range preps {
					if r == uint32(v%n) {
						log = append(log, prep)
					} else {
						log = append(log, newTestComm(impl, r, prep, uint64(i)+1))
					}
				}
				for l := 0; l < len(log); l++ {
					logs[r][v] = append(logs[r][v], log[:l])
				}
			} else {
				// TODO: Test against logs with NewView messages
				for _, log := range logs[r][v-1] {
					lastCV := lastLogCV(log)
					vc := newTestVC(impl, r, v, log, randVCCert(impl, f, n, v), lastCV+1)
					logs[r][v] = append(logs[r][v], messages.MessageLog{vc})
				}
			}
		}
	}

	for i, logs := range logs {
		r := uint32(i)
		for i, logs := range logs {
			v := uint64(i)
			for i, log := range logs {
				t.Run(fmt.Sprintf("Replica=%d/View=%d/Log=%d", r, v, i), func(t *testing.T) {
					testViewChange(t, impl, r, v, log, randVCCert(impl, f, n, v))
				})
			}
		}
	}
}

func testViewChange(t *testing.T, impl messages.MessageImpl, r uint32, v uint64, log messages.MessageLog, vcCert messages.ViewChangeCert) {
	lastCV := lastLogCV(log)

	t.Run("Fields", func(t *testing.T) {
		vc := impl.NewViewChange(r, v, log, vcCert)
		require.Equal(t, r, vc.ReplicaID())
		require.Equal(t, v, vc.NewView())
		requireMsgLogEqual(t, log, vc.MessageLog())
		requireVCCertEqual(t, vcCert, vc.ViewChangeCert())
	})
	t.Run("SetUI", func(t *testing.T) {
		vc := impl.NewViewChange(r, v, log, vcCert)
		ui := newTestUI(lastCV+1, messages.AuthenBytes(vc))
		vc.SetUI(ui)
		require.Equal(t, ui, vc.UI())
	})
	t.Run("Marshaling", func(t *testing.T) {
		vc := newTestVC(impl, r, v, log, vcCert, lastCV+1)
		requireVCEqual(t, vc, remarshalMsg(impl, vc).(messages.ViewChange))
	})
}

func newTestVC(impl messages.MessageImpl, r uint32, v uint64, log messages.MessageLog, vcCert messages.ViewChangeCert, cv uint64) messages.ViewChange {
	vc := impl.NewViewChange(r, v, log, vcCert)
	vc.SetUI(newTestUI(cv, messages.AuthenBytes(vc)))
	return vc
}

func lastLogCV(log messages.MessageLog) uint64 {
	var lastCV uint64
	if len(log) != 0 {
		lastCV = log[len(log)-1].UI().Counter
	}
	return lastCV
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
