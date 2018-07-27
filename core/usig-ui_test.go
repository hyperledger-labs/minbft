// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
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

package minbft

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nec-blockchain/minbft/api"
	"github.com/nec-blockchain/minbft/messages"
	"github.com/nec-blockchain/minbft/usig"

	mock_api "github.com/nec-blockchain/minbft/api/mocks"
	mock_messages "github.com/nec-blockchain/minbft/messages/mocks"
)

func TestUIVerifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	setExpectation := func(ok bool) (api.Authenticator, messages.MessageWithUI, *usig.UI) {
		msg := mock_messages.NewMockMessageWithUI(ctrl)
		authen := mock_api.NewMockAuthenticator(ctrl)

		replicaID := rand.Uint32()
		cv := rand.Uint64()

		payload := []byte(fmt.Sprintf("MessageWithUI{replicaID: %d}", replicaID))
		ui := &usig.UI{Counter: cv, Cert: []byte{}}
		uiBytes, _ := ui.MarshalBinary()
		msg.EXPECT().ReplicaID().Return(replicaID).AnyTimes()
		msg.EXPECT().Payload().Return(payload).AnyTimes()
		msg.EXPECT().UIBytes().Return(uiBytes).AnyTimes()

		var err error
		if !ok {
			err = fmt.Errorf("USIG certificate invalid")
		}
		authen.EXPECT().VerifyMessageAuthenTag(
			api.USIGAuthen, replicaID, payload, uiBytes,
		).Return(err).AnyTimes()
		return authen, msg, ui
	}

	// Correct UI
	authen, msg, expectedUI := setExpectation(true)
	verifier := makeUIVerifier(authen)
	actualUI, err := verifier(msg)
	assert.NoError(t, err)
	assert.Equal(t, expectedUI, actualUI)

	// Failed USIG certificate verification
	authen, msg, _ = setExpectation(false)
	verifier = makeUIVerifier(authen)
	actualUI, err = verifier(msg)
	assert.Error(t, err)
	assert.Nil(t, actualUI)
}

func TestUIAcceptor(t *testing.T) {
	cases := []struct {
		desc      string
		replicaID int
		cv        int
		invalid   bool
		ok        bool
		new       bool
	}{
		{
			desc: "Invalid (zero) counter value",
			cv:   0,
		}, {
			desc:    "Invalid USIG certificate",
			cv:      1,
			invalid: true,
		}, {
			desc: "First UI with too advanced counter value",
			cv:   2,
		}, {
			desc: "First valid UI",
			cv:   1,
			ok:   true,
			new:  true,
		}, {
			desc: "The same UI again",
			cv:   1,
			ok:   true,
		}, {
			desc: "Too advanced counter value",
			cv:   3,
		}, {
			desc:      "First valid UI from another replica",
			replicaID: 1,
			cv:        1,
			ok:        true,
			new:       true,
		}, {
			desc:      "Second valid UI from another replica",
			replicaID: 1,
			cv:        2,
			ok:        true,
			new:       true,
		}, {
			desc: "Second valid UI",
			cv:   2,
			ok:   true,
			new:  true,
		}, {
			desc: "Previous valid UI",
			cv:   1,
			ok:   true,
		}, {
			desc: "Third valid UI",
			cv:   3,
			ok:   true,
			new:  true,
		},
	}

	acceptor := makeUIAcceptor(fakeVerifier)
	for _, c := range cases {
		new, err := acceptor(&fakeMsgWithUI{c.replicaID, c.cv, !c.invalid})
		if c.ok {
			require.NoErrorf(t, err, c.desc)
		} else {
			require.Error(t, err, c.desc)
		}
		require.Equal(t, c.new, new, c.desc)
	}
}

func TestUIAcceptorConcurrent(t *testing.T) {
	const nrConcurrent = 5
	const nrUIs = 10

	wg := new(sync.WaitGroup)
	wg.Add(nrConcurrent)

	acceptor := makeUIAcceptor(fakeVerifier)
	for id := 0; id < nrConcurrent; id++ {
		go func(workerID int) {
			for cv := 1; cv <= nrUIs; cv++ {
				new, err := acceptor(&fakeMsgWithUI{workerID, cv, true})
				assertMsg := fmt.Sprintf("Worker %d, UI %d", workerID, cv)
				assert.NoErrorf(t, err, assertMsg)
				assert.True(t, new, assertMsg)
			}
			defer wg.Done()
		}(id)
	}

	wg.Wait()
}

func TestUIAssigner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	msg := mock_messages.NewMockMessageWithUI(ctrl)
	authen := mock_api.NewMockAuthenticator(ctrl)

	payload := make([]byte, 1)
	uiBytes := make([]byte, 1)
	rand.Read(payload)
	rand.Read(uiBytes)
	msg.EXPECT().Payload().Return(payload).AnyTimes()

	assignUI := makeUIAssigner(authen)

	err := fmt.Errorf("Failed to generate UI")
	authen.EXPECT().GenerateMessageAuthenTag(api.USIGAuthen, payload).Return(nil, err)
	assert.Panics(t, func() { assignUI(msg) })

	authen.EXPECT().GenerateMessageAuthenTag(api.USIGAuthen, payload).Return(uiBytes, nil)
	msg.EXPECT().AttachUI(uiBytes)
	assignUI(msg)
}

type fakeMsgWithUI struct {
	replicaID int
	cv        int
	valid     bool
}

func (m *fakeMsgWithUI) ReplicaID() uint32  { return uint32(m.replicaID) }
func (m *fakeMsgWithUI) Payload() []byte    { return nil }
func (m *fakeMsgWithUI) UIBytes() []byte    { return nil }
func (m *fakeMsgWithUI) AttachUI(ui []byte) {}

func fakeVerifier(msg messages.MessageWithUI) (ui *usig.UI, err error) {
	m := msg.(*fakeMsgWithUI)
	ui = &usig.UI{Counter: uint64(m.cv)}
	if !m.valid {
		err = fmt.Errorf("USIG certificate invalid")
	}
	return
}
