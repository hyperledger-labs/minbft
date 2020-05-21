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

package messages

import (
	"bytes"
	"encoding/binary"
	"io"

	hash "crypto/sha256"
)

// AuthenBytes returns serialized representation of message's
// authenticated content.
func AuthenBytes(m Message) []byte {
	buf := &bytes.Buffer{}

	var tag string
	switch m.(type) {
	case Request:
		tag = "REQUEST"
	case Reply:
		tag = "REPLY"
	case Prepare:
		tag = "PREPARE"
	case Commit:
		tag = "COMMIT"
	case ReqViewChange:
		tag = "REQ-VIEW-CHANGE"
	default:
		panic("unknown message type")
	}

	_, _ = buf.Write([]byte(tag))
	writeAuthenBytes(buf, m)

	return buf.Bytes()
}

func writeAuthenBytes(buf io.Writer, m Message) {
	switch m := m.(type) {
	case Request:
		_ = binary.Write(buf, binary.BigEndian, m.Sequence())
		_, _ = buf.Write(hashsum(m.Operation()))
	case Reply:
		_ = binary.Write(buf, binary.BigEndian, m.ClientID())
		_ = binary.Write(buf, binary.BigEndian, m.Sequence())
		_, _ = buf.Write(hashsum(m.Result()))
	case Prepare:
		_ = binary.Write(buf, binary.BigEndian, m.View())
		req := m.Request()
		_ = binary.Write(buf, binary.BigEndian, req.ClientID())
		writeAuthenBytes(buf, req)
	case Commit:
		prep := m.Prepare()
		_ = binary.Write(buf, binary.BigEndian, prep.ReplicaID())
		writeAuthenBytes(buf, prep)
		_ = binary.Write(buf, binary.BigEndian, prep.UI().Counter)
	case ReqViewChange:
		_ = binary.Write(buf, binary.BigEndian, m.NewView())
	default:
		panic("unknown message type")
	}
}

func hashsum(data []byte) []byte {
	h := hash.New()
	_, _ = h.Write(data)
	return h.Sum(nil)
}
