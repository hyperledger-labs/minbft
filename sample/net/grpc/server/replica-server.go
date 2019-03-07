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

// Package server implements a counterpart for serving incoming gRPC
// connections initiated by gRPC-based ReplicaConnector and connects
// it directly to an instance of MessageStreamHandler interface
package server

import (
	"fmt"
	"io"
	"log"
	"net"

	"golang.org/x/sync/errgroup"

	"google.golang.org/grpc"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/sample/net/grpc/proto"
)

// ReplicaServer implements a gRPC server to serve incoming
// connections from ReplicaConnector of this package
type ReplicaServer struct {
	replica    api.MessageStreamHandler
	grpcServer *grpc.Server
}

var _ proto.ChannelServer = (*ReplicaServer)(nil)

// New creates a new instance of ReplicaServer using the specified
// instance of MessageStreamHandler to connect incoming requests with.
func New(replica api.MessageStreamHandler) *ReplicaServer {
	return &ReplicaServer{replica: replica}
}

// ListenAndServe listens on the TCP network address and then serves
// incoming connections.
func (s *ReplicaServer) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Error listing on %s: %s", addr, err)
	}
	s.grpcServer = grpc.NewServer()
	proto.RegisterChannelServer(s.grpcServer, s)

	err = s.grpcServer.Serve(lis)
	if err != nil {
		return fmt.Errorf("Error serving: %s", err)
	}
	return nil
}

// Stop stops the server. It immediately closes all open connections
// and listeners.
func (s *ReplicaServer) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
		s.grpcServer = nil
	}
}

// Chat implements a corresponding gRPC server interface method
func (s *ReplicaServer) Chat(stream proto.Channel_ChatServer) error {
	in := make(chan []byte)
	out := s.replica.HandleMessageStream(in)

	eg := new(errgroup.Group)

	eg.Go(func() error {
		defer close(in)
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				err = fmt.Errorf("error receiving from replica server stream: %s", err)
				log.Println(err)
				return err
			}

			in <- msg.Payload
		}
		return nil
	})

	eg.Go(func() error {
		for msg := range out {
			err := stream.Send(&proto.Message{Payload: msg})
			if err != nil {
				err = fmt.Errorf("error sending to replica server stream: %s", err)
				log.Println(err)
				return err
			}
		}
		return nil
	})

	return eg.Wait()
}
