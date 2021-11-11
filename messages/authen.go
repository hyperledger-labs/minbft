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
	case ViewChange:
		tag = "VIEW-CHANGE"
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
		prop := m.Proposal()
		_ = binary.Write(buf, binary.BigEndian, prop.ReplicaID())
		_, _ = buf.Write(AuthenBytes(prop))
		_ = binary.Write(buf, binary.BigEndian, prop.UI().Counter)
	case ReqViewChange:
		_ = binary.Write(buf, binary.BigEndian, m.NewView())
	case ViewChange:
		_ = binary.Write(buf, binary.BigEndian, m.NewView())
		// There is no need to authenticate any other message
		// content. Messages in the message log are certified
		// with USIG and therefore cannot be reordered,
		// omitted, or duplicated without it being detected,
		// whereas any valid view-change certificate is
		// equivalent to any other valid one. Moreover,
		// messages in the message log and view-change
		// certificate are authenticated on their own.
	default:
		panic("unknown message type")
	}
}

func hashsum(data []byte) []byte {
	h := hash.New()
	_, _ = h.Write(data)
	return h.Sum(nil)
}
