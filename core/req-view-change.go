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

package minbft

import (
	"fmt"

	"github.com/hyperledger-labs/minbft/messages"
)

// reqViewChangeValidator validates a ReqViewChangeMessage.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type reqViewChangeValidator func(rvc messages.ReqViewChange) error

func makeReqViewChangeValidator(verifySignature messageSignatureVerifier) reqViewChangeValidator {
	return func(rvc messages.ReqViewChange) error {
		if rvc.NewView() < 1 {
			return fmt.Errorf("Invalid new view number")
		}

		if err := verifySignature(rvc); err != nil {
			return fmt.Errorf("Signature is not valid: %s", err)
		}

		return nil
	}
}
