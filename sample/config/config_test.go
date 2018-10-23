// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Wenting Li <wenting.li@neclab.eu>
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

package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hyperledger-labs/minbft/api"

	"github.com/stretchr/testify/assert"
)

type ConfigTester interface {
	LoadConfig(filePath string)
	IsInitialized() bool

	api.Configer
}

var configers = []ConfigTester{New()}

var yamlExample = []byte(`
protocol:
    "n": 3
    f: 1
    checkpointPeriod: 10
    logsize: 20

    timeout:
        request: 2s
        viewchange: 3s
`)

// initExampleFile writes the configuration example `cfgExample` to a temporary
// file which specifies the cfg type with proper file extension `fileExt`. It
// returns the path to the temporary configuration file and the cleanup function.
func initExampleFile(t *testing.T, fileExt string, cfgExample []byte) (string, func()) {
	dir, err := ioutil.TempDir("", "minbft_config_test")
	assert.NoError(t, err, "Failed to create temporary directory.")

	testfn := filepath.Join(dir, "test-config."+fileExt)
	err = ioutil.WriteFile(testfn, cfgExample, 0666)
	assert.NoError(t, err, "Failed to write to test config file.")

	return testfn, func() {
		defer os.RemoveAll(dir)
	}
}

func testParseParam(t *testing.T) {
	testfn, cleanup := initExampleFile(t, "yaml", yamlExample)
	defer cleanup()

	for _, cfg := range configers {
		cfg.LoadConfig(testfn)
		assert.True(t, cfg.IsInitialized())

		assert.Equal(t, uint32(3), cfg.N())
		assert.Equal(t, uint32(1), cfg.F())
		assert.Equal(t, uint32(10), cfg.CheckpointPeriod())
		assert.Equal(t, uint32(20), cfg.Logsize())
		assert.Equal(t, 2*time.Second, cfg.TimeoutRequest())
		assert.Equal(t, 3*time.Second, cfg.TimeoutViewChange())
	}
}

func TestConfig(t *testing.T) {
	t.Run("ParseParam", testParseParam)
}
