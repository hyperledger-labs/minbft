// Copyright © 2018 NEC Laboratories Europe GmbH.
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

package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/a8m/envsubst"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/hyperledger-labs/minbft/api"
	minbft "github.com/hyperledger-labs/minbft/core"
	authen "github.com/hyperledger-labs/minbft/sample/authentication"
	"github.com/hyperledger-labs/minbft/sample/config"
	"github.com/hyperledger-labs/minbft/sample/net/grpc/connector"
	"github.com/hyperledger-labs/minbft/sample/net/grpc/server"
	"github.com/hyperledger-labs/minbft/sample/requestconsumer"
)

const (
	defConsensusCfgFile = "consensus.yaml"
	defKeysFile         = "keys.yaml"
	defUsigEnclaveFile  = "$GOPATH/src/github.com/hyperledger-labs/minbft/" +
		"usig/sgx/enclave/libusig.signed.so"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [id]",
	Short: "Run replica instance",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("Failed to parse replica ID "+
					"from positional argument: %s", err)
			}
			viper.Set("replica.id", id)
		}

		return run()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().Int("id", 0, "ID of replica")
	must(viper.BindPFlag("replica.id",
		runCmd.Flags().Lookup("id")))

	runCmd.Flags().StringP("usig-enclave-file", "u",
		defUsigEnclaveFile, "USIG enclave file")
	must(viper.BindPFlag("usig.enclaveFile",
		runCmd.Flags().Lookup("usig-enclave-file")))
}

type replicaStack struct {
	api.ReplicaConnector
	api.Authenticator
	api.RequestConsumer
}

func run() error {
	id := uint32(viper.GetInt("replica.id"))

	usigEnclaveFile, err := envsubst.String(viper.GetString("usig.enclaveFile"))
	if err != nil {
		return fmt.Errorf("Failed to parse USIG enclave filename: %s", err)
	}

	keysFile, err := os.Open(viper.GetString("keys"))
	if err != nil {
		return fmt.Errorf("Failed to open keyset file: %s", err)
	}

	auth, err := authen.NewWithSGXUSIG([]api.AuthenticationRole{api.ReplicaAuthen, api.USIGAuthen}, id, keysFile, usigEnclaveFile)
	if err != nil {
		return fmt.Errorf("Failed to create authenticator: %s", err)
	}

	cfg := config.New()
	cfg.LoadConfig(viper.GetString("consensusConf"))

	ledger := requestconsumer.NewSimpleLedger()

	peerAddrs := make(map[uint32]string)
	for _, p := range cfg.Peers() {
		peerAddrs[uint32(p.ID)] = p.Addr
	}

	replicaConnector := connector.New()
	replica, err := minbft.New(id, cfg, &replicaStack{replicaConnector, auth, ledger})
	if err != nil {
		return fmt.Errorf("Failed to create replica instance: %s", err)
	}
	replicaServer := server.New(replica)

	// We must start a replica server before attempting to connect
	// to other replicas, because other replicas will do the same.
	// Otherwise, all replicas will block forever waiting for each
	// other to start serving incoming connections.
	srvErrChan := make(chan error)
	listenAddr := peerAddrs[id]
	go func() {
		defer replicaServer.Stop()
		if err := replicaServer.ListenAndServe(listenAddr); err != nil {
			err = fmt.Errorf("Network server failed: %s", err)
			fmt.Println(err)
			srvErrChan <- err
		}
	}()

	delete(peerAddrs, id) // avoid connecting back to this replica
	dialOpts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithBlock()}
	if err := replicaConnector.ConnectManyReplicas(peerAddrs, dialOpts...); err != nil {
		return fmt.Errorf("Failed to connect to peers: %s", err)
	}

	if err := replica.Start(); err != nil {
		return fmt.Errorf("Failed to start replica: %s", err)
	}

	return <-srvErrChan
}
