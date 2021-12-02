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
	"hash/crc32"
	"io"
	"math/rand"
	"testing"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
	"github.com/stretchr/testify/require"
)

func randBytes() []byte {
	return []byte{byte(rand.Int())} // nolint:gosec
}

func testSig(data []byte) []byte {
	cs := crc32.NewIEEE()
	_, _ = cs.Write(data)
	return cs.Sum(nil)
}

func remarshalMsg(impl messages.MessageImpl, msg messages.Message) messages.Message {
	msgBytes, err := msg.MarshalBinary()
	if err != nil {
		panic(err)
	}
	msg2, err := impl.NewFromBinary(msgBytes)
	if err != nil {
		panic(err)
	}
	return msg2
}

func newTestUI(cv uint64, data []byte) *usig.UI {
	return &usig.UI{
		Counter: cv,
		Cert:    testSig([]byte(fmt.Sprintf("%d:%x", cv, data))),
	}
}

func randUI(data []byte) *usig.UI {
	return newTestUI(rand.Uint64(), data)
}

func requireCertMsgEqual(t *testing.T, m1, m2 messages.CertifiedMessage) {
	switch m1 := m1.(type) {
	case messages.Prepare:
		m2, ok := m2.(messages.Prepare)
		require.True(t, ok)
		requirePrepEqual(t, m1, m2)
	case messages.Commit:
		m2, ok := m2.(messages.Commit)
		require.True(t, ok)
		requireCommEqual(t, m1, m2)
	case messages.ViewChange:
		m2, ok := m2.(messages.ViewChange)
		require.True(t, ok)
		requireVCEqual(t, m1, m2)
	case messages.NewView:
		m2, ok := m2.(messages.NewView)
		require.True(t, ok)
		requireNVEqual(t, m1, m2)
	default:
		panic("Unexpected message type")
	}
}

func lastLogCV(log messages.MessageLog) (cv uint64) {
	if m := lastLogMsg(log); m != nil {
		cv = m.UI().Counter
	}
	return
}

func lastLogMsg(log messages.MessageLog) messages.CertifiedMessage {
	if len(log) == 0 {
		return nil
	}
	return log[len(log)-1]
}

// generateMessageLogs returns a channel that receives possible sets
// of message logs, indexed by replica ID which, for the view number v
// derived from possible protocol histories preparing and committing
// the supplied sequence of requests among the views, given the max
// number of faulty replicas f and the total number of replicas n.
func generateMessageLogs(impl messages.MessageImpl, f, n uint32, v uint64, reqs []messages.Request) <-chan []messages.MessageLog {
	out := make(chan []messages.MessageLog, 1)

	go func() {
		defer close(out)

		// Check for base case: populate initial message logs
		if v == 0 {
			logs := make([]messages.MessageLog, n)
			out <- populateLogs(impl, n, v, logs, reqs)
			return
		}
		p := uint32(v % uint64(n))

		// Vary number of requests in the last view
		for k := len(reqs); k >= 0; k-- {
			// Generate message logs for the previous view
			for logs := range generateMessageLogs(impl, f, n, v-1, reqs[:k]) {
				// Derive possible partial logs
				for logs := range generatePartialLogs((p+n-1)%n, n-f-1, logs) {
					// Terminate the previous view
					logs, vcs := terminateMessageLogs(impl, f, n, v-1, logs)
					reqs := reqs[k:]    // remaining requests for the last view
					if len(reqs) == 0 { // no request in the last view
						out <- logs // empty view is possible
					}

					// Derive possible logs with NewView and remaining requests
					for nv := range generateNewViewMessages(impl, f, n, v, vcs) {
						logs := append([]messages.MessageLog{}, logs...)
						logs[p] = messages.MessageLog{nv}
						out <- populateLogs(impl, n, v, logs, reqs)
					}
				}
			}
		}
	}()

	return out
}

// generateNewViewMessages returns a channel that receives possible
// NewView messages for the view number v derived from the supplied
// set of ViewChange messages, indexed by replica ID, given the max
// number of faulty replicas f and the total number of replicas n.
func generateNewViewMessages(impl messages.MessageImpl, f, n uint32, v uint64, vcs []messages.ViewChange) <-chan messages.NewView {
	out := make(chan messages.NewView, 1)

	go func() {
		defer close(out)

		p := uint32(v % uint64(n))
		cv := vcs[p].UI().Counter + 1
		for nvCert := range generateNewViewCertificates(impl, f, n, v, vcs) {
			out <- newTestNV(impl, p, v, nvCert, cv)
		}
	}()

	return out
}

// generateNewViewCertificates returns a channel that receives
// possible new-view certificates for the view number v derived from
// the supplied set of ViewChange messages, indexed by replica ID,
// given the max number of faulty replicas f and the total number of
// replicas n.
func generateNewViewCertificates(impl messages.MessageImpl, f, n uint32, v uint64, vcs []messages.ViewChange) <-chan messages.NewViewCert {
	out := make(chan messages.NewViewCert, 1)

	go func() {
		defer close(out)

		p := uint32(v % uint64(n))
		for q := range extendQuorum(n-f-1, n, map[uint32]bool{p: true}) {
			nvCert := make(messages.NewViewCert, 0, n-f)
			for r := range q {
				nvCert = append(nvCert, vcs[r])
			}
			out <- nvCert
		}
	}()

	return out
}

// terminateMessageLogs replaces the message logs in the supplied set,
// indexed by replica ID, with ViewChange messages for the next view,
// given the max number of faulty replicas f, the total number of
// replicas n, and the current view number v.
func terminateMessageLogs(impl messages.MessageImpl, f, n uint32, v uint64, logs []messages.MessageLog) ([]messages.MessageLog, []messages.ViewChange) {
	logs = append([]messages.MessageLog{}, logs...)
	vcs := make([]messages.ViewChange, n)
	for r := uint32(0); r < n; r++ {
		cv := lastLogCV(logs[r]) + 1
		vcs[r] = newTestVC(impl, r, v+1, logs[r], randVCCert(impl, f, n, v+1), cv)
		logs[r] = messages.MessageLog{vcs[r]}
	}
	return logs, vcs
}

// generatePartialLogs returns a channel that receives possible sets
// of partial message logs derived from the supplied set of message
// logs, indexed by replica ID, by possibly truncating different
// message logs of up to t replicas, except the one of the replica p.
func generatePartialLogs(p, t uint32, logs []messages.MessageLog) <-chan []messages.MessageLog {
	out := make(chan []messages.MessageLog, 1)

	go func() {
		defer close(out)

		// Check for base case: no more logs to truncate
		if len(logs) == 0 {
			out <- make([]messages.MessageLog, 0, cap(logs))
			return
		}

		r := uint32(len(logs) - 1)
		logs, l := logs[:r], logs[r]

		// Make partial logs preserving the last replica's log
		for logs := range generatePartialLogs(p, t, logs) {
			out <- append(logs, l)
		}

		// Make partial logs possibly truncating the last replica's log
		if t > 0 && r != p {
			for logs := range generatePartialLogs(p, t-1, logs) {
				for k := len(l); k > 0; k-- {
					logs := append(make([]messages.MessageLog, 0, cap(logs)), logs...)
					out <- append(logs, l[:k-1])
				}
			}
		}
	}()

	return out
}

// extendQuorum returns a channel that receives possible extensions of
// the supplied replica set q with t more replica IDs, given the total
// number of replicas n.
func extendQuorum(t, n uint32, q map[uint32]bool) <-chan map[uint32]bool {
	out := make(chan map[uint32]bool, 1)

	go func() {
		defer close(out)

		// Check for base case: no more replica IDs to add
		if t == 0 {
			q2 := make(map[uint32]bool, len(q))
			for k, v := range q {
				q2[k] = v
			}
			out <- q2
			return
		}

		// Make replica sets without the replica ID n-1
		if t < n {
			for q := range extendQuorum(t, n-1, q) {
				out <- q
			}
		}

		// Make replica sets with the replica ID n-1
		r := n - 1
		if !q[r] {
			for q := range extendQuorum(t-1, n-1, q) {
				q[r] = true
				out <- q
			}
		}
	}()

	return out
}

// populateLogs extends the supplied set of message logs, indexed by
// replica ID, with Prepare/Commit messages for the supplied sequence
// of requests, given the total number of replicas n and the current
// view number v.
func populateLogs(impl messages.MessageImpl, n uint32, v uint64, logs []messages.MessageLog, reqs []messages.Request) []messages.MessageLog {
	p := uint32(v % uint64(n))

	// Populate primary log with Prepare messages
	props := logs[p]
	props = append(make(messages.MessageLog, 0, len(props)+len(reqs)), props...)
	for i, cv := 0, lastLogCV(props)+1; i < len(reqs); i, cv = i+1, cv+1 {
		props = append(props, newTestPrep(impl, p, v, reqs[i], cv))
	}
	logs[p] = props

	// Populate backup logs with Commit messages
	for r := uint32(0); r < n; r++ {
		if r != p {
			logs[r] = append(make(messages.MessageLog, 0, len(logs[r])+len(props)), logs[r]...)
			for i, cv := 0, lastLogCV(logs[r])+1; i < len(props); i, cv = i+1, cv+1 {
				logs[r] = append(logs[r], newTestComm(impl, r, props[i], cv))
			}
		}
	}

	return logs
}

func writeLogAsString(w io.Writer, log messages.MessageLog, delim string) {
	if len(log) != 0 {
		m, log := log[0], log[1:]
		switch m := m.(type) {
		case messages.NewView:
			r := m.ReplicaID()
			for _, vc := range m.NewViewCert() {
				if vc.ReplicaID() == r {
					writeLogAsString(w, messages.MessageLog{vc}, delim)
					io.WriteString(w, delim)
				}
			}
		case messages.ViewChange:
			if log := m.MessageLog(); len(log) > 0 {
				writeLogAsString(w, log, delim)
				io.WriteString(w, delim)
			}
		}
		io.WriteString(w, messages.Stringify(m))
		for _, m := range log {
			io.WriteString(w, delim)
			io.WriteString(w, messages.Stringify(m))
		}
	}
}
