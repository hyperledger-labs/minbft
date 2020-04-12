// Copyright (c) 2020 NEC Solution Innovators, Ltd.
//
// Authors: Naoya Horiguchi <naoya.horiguchi@nec.com>
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

package minbft_test

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"github.com/hyperledger-labs/minbft/sample/config"
)

const cfgBftTemplate = `
protocol:
  "n": {{.N}}
  f: {{getF .N}}
  checkpointPeriod: 10
  logsize: 20
  timeout:
    request: 2s
    viewchange: 3s
debug:
  selectiveignorantreplicas:
    {{range $k, $v := .Ignore}}{{$k}}:{{range $s := $v}} {{$s}}{{end}}
    {{end}}
`

// createTestCfgBft creates config file for Byzantine faulty cases.
func createTestCfgBft(numReplica int, ignore map[string][]int) []byte {
	var err error
	var testCfgBft bytes.Buffer

	t := template.New("cfgBftTemplate")
	t = t.Funcs(template.FuncMap{
		"getF": func(n int) int { return (n - 1) / 2 },
	})
	t = template.Must(t.Parse(cfgBftTemplate))
	if err = t.Execute(&testCfgBft, struct {
		N      int
		Ignore map[string][]int
	}{
		N:      numReplica,
		Ignore: ignore,
	}); err != nil {
		panic(err)
	}
	return testCfgBft.Bytes()
}

func initTestnetPeersBFT(numReplica, numClient int, ignore map[string][]int) {
	resetFixture()

	// generate config, keys
	testCfgBft := createTestCfgBft(numReplica, ignore)
	testKeys := createTestKeys(numReplica, numClient)

	cfg := config.New() // configer shared by all replicas
	err := cfg.ReadConfig(bytes.NewBuffer(testCfgBft), "yaml")
	if err != nil {
		panic(err)
	}

	makeReplicas(numReplica, testKeys, cfg)
	makeClients(numClient, testKeys, cfg)
}

func TestByzantineFault(t *testing.T) {
	testCases := []struct {
		numReplica int
		numClient  int
		ignore     map[string][]int
	}{
		{numReplica: 3, numClient: 1, ignore: map[string][]int{"2": {1}}},
		{numReplica: 3, numClient: 1, ignore: map[string][]int{"2": {0}}},
		{numReplica: 3, numClient: 1, ignore: map[string][]int{"0": {1}}},
	}
	for _, tc := range testCases {
		// setup
		initTestnetPeersBFT(tc.numReplica, tc.numClient, tc.ignore)

		t.Run(fmt.Sprintf("TestnetAcceptOneRequest/r=%d/c=%d/ignore=%v", tc.numReplica, tc.numClient, tc.ignore), testAcceptOneRequest)
	}
}
