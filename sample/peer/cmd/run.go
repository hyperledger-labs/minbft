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
	logging "github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"

	"github.com/hyperledger-labs/minbft/api"
	minbft "github.com/hyperledger-labs/minbft/core"
	authen "github.com/hyperledger-labs/minbft/sample/authentication"
	"github.com/hyperledger-labs/minbft/sample/config"
	"github.com/hyperledger-labs/minbft/sample/conn/grpc/connector"
	"github.com/hyperledger-labs/minbft/sample/conn/grpc/server"
	"github.com/hyperledger-labs/minbft/sample/requestconsumer"
)

const (
	defConsensusCfgFile = "consensus.yaml"
	defKeysFile         = "keys.yaml"
	defUsigEnclaveFile  = "libusig.signed.so"
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

	rootCmd.PersistentFlags().String("logging-level", "", "logging level")
	must(viper.BindPFlag("logging.level",
		rootCmd.PersistentFlags().Lookup("logging-level")))

	rootCmd.PersistentFlags().String("logging-file", "", "logging file")
	must(viper.BindPFlag("logging.file",
		rootCmd.PersistentFlags().Lookup("logging-file")))
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
	var listenAddr string
	for _, p := range cfg.Peers() {
		// avoid connecting back to this replica
		if uint32(p.ID) == id {
			listenAddr = p.Addr
		} else {
			peerAddrs[uint32(p.ID)] = p.Addr
		}
	}

	loggingOpts, err := getLoggingOptions()
	if err != nil {
		return fmt.Errorf("Failed to create logging options: %s", err)
	}

	conn := connector.NewReplicaSide(id)

	// XXX: The connection destination should be authenticated;
	// grpc.WithInsecure() option is passed here for simplicity.
	if err = connector.ConnectManyReplicas(conn, peerAddrs, grpc.WithInsecure()); err != nil {
		return fmt.Errorf("Failed to connect to peers: %s", err)
	}

	replica, err := minbft.New(id, cfg, &replicaStack{conn, auth, ledger}, loggingOpts...)
	if err != nil {
		return fmt.Errorf("Failed to create replica instance: %s", err)
	}

	// XXX: Incoming peer connections should be authenticated.
	// This is not yet supported by the server package.
	replicaServer := server.New(replica)

	srvErrChan := make(chan error)
	go func() {
		defer replicaServer.Stop()

		// XXX: The replica server should authenticate itself;
		// appropriate gRPC server options are omitted here
		// for simplicity.
		if err := server.ListenAndServe(replicaServer, listenAddr); err != nil {
			err = fmt.Errorf("Network server failed: %s", err)
			fmt.Println(err)
			srvErrChan <- err
		}
	}()

	return <-srvErrChan
}

func getLoggingOptions() ([]minbft.Option, error) {
	opts := []minbft.Option{}

	if viper.GetString("logging.level") != "" {
		logLevel, err := logging.LogLevel(viper.GetString("logging.level"))
		if err != nil {
			return nil, fmt.Errorf("Failed to set logging level: %s", err)
		}
		opts = append(opts, minbft.WithLogLevel(logLevel))
	}

	if viper.GetString("logging.file") != "" {
		logFile, err := os.OpenFile(viper.GetString("logging.file"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("Failed to open logging file: %s", err)
		}
		opts = append(opts, minbft.WithLogFile(logFile))
	}

	return opts, nil
}
