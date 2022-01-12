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

package testing

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/messages"
)

func DoTestNewView(t *testing.T, impl messages.MessageImpl) {
	const f = 1
	const n = 3
	const maxNrViews = 3
	const maxNrRequests = 2

	reqs := make([]messages.Request, maxNrRequests)
	for i := range reqs {
		reqs[i] = MakeTestReq(impl, 0, uint64(i), nil)
	}

	for k := len(reqs); k >= 0; k-- {
		reqs := reqs[:k]
		for v := uint64(1); v < maxNrViews; v++ {
			p := uint32(v % uint64(n))
			i := 0
			for logs := range GenerateMessageLogs(impl, f, n, v-1, reqs) {
				_, vcs := TerminateMessageLogs(impl, f, n, v-1, logs)
				cv := vcs[p].UI().Counter
				j := 0
				for nvCert := range GenerateNewViewCertificates(impl, f, n, v, vcs) {
					t.Run(fmt.Sprintf("NrRequests=%d/View=%d/Log=%d/Cert=%d", k, v, i, j), func(t *testing.T) {
						testNewView(t, impl, p, v, nvCert, cv)
					})
					j++
				}
				i++
			}
		}
		if testing.Short() {
			return
		}
	}
}

func testNewView(t *testing.T, impl messages.MessageImpl, r uint32, v uint64, nvCert messages.NewViewCert, cv uint64) {
	t.Run("Fields", func(t *testing.T) {
		nv := impl.NewNewView(r, v, nvCert)
		require.Equal(t, r, nv.ReplicaID())
		require.Equal(t, v, nv.NewView())
		RequireNVCertEqual(t, nvCert, nv.NewViewCert())
	})
	t.Run("SetUI", func(t *testing.T) {
		nv := impl.NewNewView(r, v, nvCert)
		ui := MakeTestUI(cv, messages.AuthenBytes(nv))
		nv.SetUI(ui)
		require.Equal(t, ui, nv.UI())
	})
	t.Run("Marshaling", func(t *testing.T) {
		nv := MakeTestNV(impl, r, v, nvCert, cv)
		RequireNVEqual(t, nv, RemarshalMsg(impl, nv).(messages.NewView))
	})
}

func MakeTestNVCert(impl messages.MessageImpl) messages.NewViewCert {
	return messages.NewViewCert{
		MakeTestVC(impl, 1, 1, nil, RandVCCert(impl, 1, 3, 1), 1),
		MakeTestVC(impl, 2, 1, nil, RandVCCert(impl, 1, 3, 1), 1),
	}
}

func MakeTestNV(impl messages.MessageImpl, r uint32, v uint64, nvCert messages.NewViewCert, cv uint64) messages.NewView {
	nv := impl.NewNewView(r, v, nvCert)
	nv.SetUI(MakeTestUI(cv, messages.AuthenBytes(nv)))
	return nv
}

func RequireNVEqual(t *testing.T, nv1, nv2 messages.NewView) {
	require.Equal(t, nv1.ReplicaID(), nv2.ReplicaID())
	require.Equal(t, nv1.NewView(), nv2.NewView())
	RequireNVCertEqual(t, nv2.NewViewCert(), nv2.NewViewCert())
	require.Equal(t, nv1.UI(), nv2.UI())
}

func RequireNVCertEqual(t *testing.T, c1, c2 messages.NewViewCert) {
	require.Equal(t, len(c1), len(c2))
	for i, vc1 := range c1 {
		RequireVCEqual(t, vc1, c2[i])
	}
}
