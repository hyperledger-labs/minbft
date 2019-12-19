// Copyright (c) 2018-2019 NEC Laboratories Europe GmbH.
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

package requestlist

import (
	"fmt"
	"sync"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	protobufMessages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

var messageImpl = protobufMessages.NewImpl()

func TestList(t *testing.T) {
	var cases []struct {
		cid  int // clientID
		seq  int
		add  bool
		rm   bool
		list map[int]int // clientID -> seq
	}
	casesYAML := []byte(`
- {                           list: {          }}
- {cid: 0, seq: 1, add: true, list: {0: 1      }}
- {cid: 1, seq: 2, add: true, list: {0: 1, 1: 2}}
- {cid: 0, seq: 3, add: true, list: {0: 3, 1: 2}}
- {cid: 1,         rm:  true, list: {0: 3      }}
- {cid: 1, seq: 3, add: true, list: {0: 3, 1: 3}}
- {cid: 0,         rm:  true, list: {      1: 3}}
- {cid: 1,         rm:  true, list: {          }}
`)
	if err := yaml.Unmarshal(casesYAML, &cases); err != nil {
		t.Fatal(err)
	}

	l := New()

	for i, c := range cases {
		assertMsg := fmt.Sprintf("case=%d cid=%d seq=%d add=%t rm=%t",
			i, c.cid, c.seq, c.add, c.rm)
		if c.add {
			l.Add(messageImpl.NewRequest(uint32(c.cid), uint64(c.seq), nil))
		}
		if c.rm {
			l.Remove(uint32(c.cid))
		}
		msgs := l.All()
		require.Len(t, msgs, len(c.list), assertMsg)
		for _, m := range msgs {
			cid := int(m.ClientID())
			seq := m.Sequence
			require.EqualValues(t, c.list[cid], seq)
		}
	}
}

func TestListConcurrent(t *testing.T) {
	const nrConcurrent = 3
	const nrRequests = 13

	l := New()
	wg := new(sync.WaitGroup)

	wg.Add(nrConcurrent)
	for i := 0; i < nrConcurrent; i++ {
		cid := i

		go func() {
			defer wg.Done()

			for seq := 1; seq <= nrRequests; seq++ {
				m := messageImpl.NewRequest(uint32(cid), uint64(seq), nil)
				l.Add(m)

				list := l.All()
				assert.Contains(t, list, m)

				l.Remove(uint32(cid))

				list = l.All()
				assert.NotContains(t, list, m)
			}
		}()
	}

	wg.Wait()
	assert.Empty(t, l.All())
}
