package main

import (
	"log"
	"net"
	"net/http"
	"os"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	serverImpl := newWatcherServer()

	startHTTPServer(serverImpl)
	startGRPCServer(serverImpl)
}

func startHTTPServer(serverImpl *watcherServer) {
	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	go func() {
		log.Printf("HTTP server listening at %s", httpAddr)
		if err := http.ListenAndServe(httpAddr, newRESTServer(serverImpl)); err != nil {
			log.Fatalf("failed to serve HTTP API: %v", err)
		}
	}()
}

func startGRPCServer(serverImpl *watcherServer) {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(serverImpl.unaryAuthInterceptor),
		grpc.StreamInterceptor(serverImpl.streamAuthInterceptor),
	)
	pb.RegisterWarframeMarketWatcherServer(s, serverImpl)
	reflection.Register(s)

	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC API: %v", err)
	}
}
