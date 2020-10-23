// Copyright © 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
//
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
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/client"
	"github.com/hyperledger-labs/minbft/common/logger"
	authen "github.com/hyperledger-labs/minbft/sample/authentication"
	"github.com/hyperledger-labs/minbft/sample/config"
	"github.com/hyperledger-labs/minbft/sample/conn/grpc/connector"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// requestCmd represents the request command
var requestCmd = &cobra.Command{
	Use:   "request [request...]",
	Short: "Submit requests to replicas",
	Long: `
Submit a series of requests to the consensus network, wait for it to be processed
and output the result. The requests are one or more string arguments.
If no string argument is given, requests are constructed from standard input, where
each line is passed to a separate request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := requests(args)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(requestCmd)

	requestCmd.Flags().Int("id", 0, "ID of the client")
	must(viper.BindPFlag("client.id",
		requestCmd.Flags().Lookup("id")))
	requestCmd.Flags().String("timeout", "0", "Timeout for the request")
	must(viper.BindPFlag("client.timeout", requestCmd.Flags().Lookup("timeout")))
}

type clientStack struct {
	api.Authenticator
	api.ReplicaConnector
}

func request(client client.Client, arg string) {
	var timeoutChan <-chan time.Time
	timeout := viper.GetDuration("client.timeout")

	if timeout > 0 {
		timeoutChan = time.After(timeout)
	}

	select {
	case res := <-client.Request([]byte(arg)):
		fmt.Println("Reply:", string(res))
	case <-timeoutChan:
		fmt.Println("Client Request timer expired.")
		os.Exit(1)
	}
}

func requests(args []string) ([]byte, error) {
	id := uint32(viper.GetInt("client.id"))

	keysFile, err := os.Open(viper.GetString("keys"))
	if err != nil {
		return nil, fmt.Errorf("failed to open keyset file: %s", err)
	}

	auth, err := authen.New([]api.AuthenticationRole{api.ClientAuthen}, id, keysFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator: %s", err)
	}

	cfg := config.New()
	cfg.LoadConfig(viper.GetString("consensusConf"))

	loggingOpts, err := getLoggingOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create logging options: %s", err)
	}
	clientLogger := logger.NewClientLogger(id, loggingOpts...)
	clientOptions := client.WithLogger(clientLogger)

	peerAddrs := make(map[uint32]string)
	for _, p := range cfg.Peers() {
		peerAddrs[uint32(p.ID)] = p.Addr
	}

	conn := connector.NewClientSide()

	// XXX: The connection destination should be authenticated;
	// grpc.WithInsecure() option is passed here for simplicity.
	err = connector.ConnectManyReplicas(conn, peerAddrs, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peers: %s", err)
	}

	client, err := client.New(id, cfg.N(), cfg.F(), clientStack{auth, conn}, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create client instance: %s", err)
	}

	if len(args) > 0 {
		for _, arg := range args {
			request(client, arg)
		}
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			request(client, scanner.Text())
		}
	}

	return nil, nil
}
