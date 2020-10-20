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
	"github.com/hyperledger-labs/minbft/sample/conn/grpc/proto"
)

// ReplicaServer implements a gRPC server to serve incoming
// connections from ReplicaConnector of this package.
//
// Server method serves incoming connection on the supplied listener.
// It blocks and returns either on error or if Stop method is called.
//
// Stop method gracefully stops the server. It immediately closes all
// open connections and listeners.
type ReplicaServer interface {
	Serve(lis net.Listener, serverOpts ...grpc.ServerOption) error
	Stop()
}

// ListenAndServe helps to start a server on a TCP address. It starts
// listening on a given TCP address and serving incoming connections.
func ListenAndServe(s ReplicaServer, addr string, serverOpts ...grpc.ServerOption) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("error listening on %s: %s", addr, err)
	}
	return s.Serve(lis)
}

type server struct {
	replica    api.ConnectionHandler
	grpcServer *grpc.Server
}

// New creates a new instance of ReplicaServer using the specified
// replica instance to connect incoming requests with.
func New(replica api.ConnectionHandler) ReplicaServer {
	return &server{replica: replica}
}

func (s *server) Serve(lis net.Listener, serverOpts ...grpc.ServerOption) error {
	s.grpcServer = grpc.NewServer(serverOpts...)
	proto.RegisterChannelServer(s.grpcServer, s)

	err := s.grpcServer.Serve(lis)
	if err != nil {
		return fmt.Errorf("error serving: %s", err)
	}
	return nil
}

func (s *server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
		s.grpcServer = nil
	}
}

func (s *server) ClientChat(stream proto.Channel_ClientChatServer) error {
	in := make(chan []byte)
	sh := s.replica.ClientMessageStreamHandler()
	out := sh.HandleMessageStream(in)

	return handleStream(stream, in, out)
}

func (s *server) PeerChat(stream proto.Channel_PeerChatServer) error {
	in := make(chan []byte)
	sh := s.replica.PeerMessageStreamHandler()
	out := sh.HandleMessageStream(in)

	return handleStream(stream, in, out)
}

type rpcStream interface {
	Send(*proto.Message) error
	Recv() (*proto.Message, error)
	grpc.ServerStream
}

func handleStream(stream rpcStream, in chan<- []byte, out <-chan []byte) error {
	eg := new(errgroup.Group)

	eg.Go(func() error {
		defer close(in)
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				err = fmt.Errorf("error receiving from server stream: %s", err)
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
				err = fmt.Errorf("error sending to server stream: %s", err)
				log.Println(err)
				return err
			}
		}
		return nil
	})

	return eg.Wait()
}
