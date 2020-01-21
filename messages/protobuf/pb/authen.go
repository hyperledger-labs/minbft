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
	_ = binary.Write(buf, binary.BigEndian, MessageType_REQUEST)
	writeAuthenBytesFromRequest(buf, m)
	return buf.Bytes()
}

// AuthenBytesFromReply returns serialized representation of
// authenticated content of Reply message.
func AuthenBytesFromReply(m *Reply) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, MessageType_REPLY)
	writeAuthenBytesFromReply(buf, m)
	return buf.Bytes()
}

// AuthenBytesFromPrepare returns serialized representation of
// authenticated content of Prepare message.
func AuthenBytesFromPrepare(m *Prepare) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, MessageType_PREPARE)
	writeAuthenBytesFromPrepare(buf, m)
	return buf.Bytes()
}

// AuthenBytesFromCommit returns serialized representation of
// authenticated content of Commit message.
func AuthenBytesFromCommit(m *Commit) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, MessageType_COMMIT)
	writeAuthenBytesFromCommit(buf, m)
	return buf.Bytes()
}

// AuthenBytesFromReqViewChange returns serialized representation of
// authenticated content of ReqViewChange messages.
func AuthenBytesFromReqViewChange(m *ReqViewChange) []byte {
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, MessageType_REQ_VIEW_CHANGE)
	writeAuthenBytesFromReqViewChange(buf, m)
	return buf.Bytes()
}

func writeAuthenBytesFromRequest(buf io.Writer, m *Request) {
	_ = binary.Write(buf, binary.BigEndian, m.GetSeq())
	_, _ = buf.Write(hashsum(m.GetOperation()))
}

func writeAuthenBytesFromReply(buf io.Writer, m *Reply) {
	_ = binary.Write(buf, binary.BigEndian, m.GetClientId())
	_ = binary.Write(buf, binary.BigEndian, m.GetSeq())
	_, _ = buf.Write(hashsum(m.GetResult()))
}

func writeAuthenBytesFromPrepare(buf io.Writer, m *Prepare) {
	req := m.GetRequest()
	_ = binary.Write(buf, binary.BigEndian, m.GetView())
	_ = binary.Write(buf, binary.BigEndian, req.GetClientId())
	writeAuthenBytesFromRequest(buf, req)
}

func writeAuthenBytesFromCommit(buf io.Writer, m *Commit) {
	prep := m.GetPrepare()
	_ = binary.Write(buf, binary.BigEndian, prep.GetReplicaId())
	writeAuthenBytesFromPrepare(buf, prep)
	_, _ = buf.Write(prep.GetUi())
}

func writeAuthenBytesFromReqViewChange(buf io.Writer, m *ReqViewChange) {
	_ = binary.Write(buf, binary.BigEndian, m.GetNewView())
}

func hashsum(data []byte) []byte {
	h := hash.New()
	_, _ = h.Write(data)
	return h.Sum(nil)
}
