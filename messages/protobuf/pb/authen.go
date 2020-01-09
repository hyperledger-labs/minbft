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

package pb

import (
	"bytes"
	"encoding/binary"
	"io"

	hash "crypto/sha256"
)

// AuthenBytesFromRequest returns serialized representation of
// authenticated content of Request message.
func AuthenBytesFromRequest(m *Request) []byte {
	buf := &bytes.Buffer{}
	writeAuthenBytesFromRequest(buf, m)
	return buf.Bytes()
}

// AuthenBytesFromReply returns serialized representation of
// authenticated content of Reply message.
func AuthenBytesFromReply(m *Reply) []byte {
	buf := &bytes.Buffer{}
	writeAuthenBytesFromReply(buf, m)
	return buf.Bytes()
}

// AuthenBytesFromPrepare returns serialized representation of
// authenticated content of Prepare message.
func AuthenBytesFromPrepare(m *Prepare) []byte {
	buf := &bytes.Buffer{}
	writeAuthenBytesFromPrepare(buf, m)
	return buf.Bytes()
}

// AuthenBytesFromCommit returns serialized representation of
// authenticated content of Commit message.
func AuthenBytesFromCommit(m *Commit) []byte {
	buf := &bytes.Buffer{}
	writeAuthenBytesFromCommit(buf, m)
	return buf.Bytes()
}

func writeAuthenBytesFromRequest(buf io.Writer, m *Request) {
	msg := m.GetMsg()
	_ = binary.Write(buf, binary.BigEndian, msg.GetSeq())
	_, _ = buf.Write(hashsum(msg.GetPayload()))
}

func writeAuthenBytesFromReply(buf io.Writer, m *Reply) {
	msg := m.GetMsg()
	_ = binary.Write(buf, binary.BigEndian, msg.GetClientId())
	_ = binary.Write(buf, binary.BigEndian, msg.GetSeq())
	_, _ = buf.Write(hashsum(msg.GetResult()))
}

func writeAuthenBytesFromPrepare(buf io.Writer, m *Prepare) {
	msg := m.GetMsg()
	req := msg.GetRequest()
	_ = binary.Write(buf, binary.BigEndian, msg.GetView())
	_ = binary.Write(buf, binary.BigEndian, req.GetMsg().GetClientId())
	writeAuthenBytesFromRequest(buf, req)
}

func writeAuthenBytesFromCommit(buf io.Writer, m *Commit) {
	msg := m.GetMsg()
	prep := msg.GetPrepare()
	_ = binary.Write(buf, binary.BigEndian, prep.GetMsg().GetReplicaId())
	writeAuthenBytesFromPrepare(buf, prep)
	_, _ = buf.Write(prep.GetReplicaUi())
}

func hashsum(data []byte) []byte {
	h := hash.New()
	_, _ = h.Write(data)
	return h.Sum(nil)
}
