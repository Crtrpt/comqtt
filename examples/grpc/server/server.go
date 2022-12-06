package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/logrusorgru/aurora"
	"github.com/wind-c/comqtt/plugin/auth/grpc/pb"
	"google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type GrpcAuthServer struct {
	pb.UnimplementedAuthenticateServer
}

func (GrpcAuthServer) Authenticate(ctx context.Context, req *pb.AuthReq) (*pb.AuthResp, error) {
	fmt.Printf("grpc auth req %v\r\n", req)
	return &pb.AuthResp{Code: bytes.Equal(req.User, []byte("test1"))}, nil
}

func (GrpcAuthServer) Acl(ctx context.Context, req *pb.ACLReq) (*pb.AuthResp, error) {
	fmt.Printf("grpc acl req %v\r\n", req)
	return &pb.AuthResp{Code: req.Topic == "test1"}, nil
}

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	server := grpc.NewServer()

	pb.RegisterAuthenticateServer(server, &GrpcAuthServer{})

	go func() {
		l, _ := net.Listen("tcp", "127.0.0.1:9001")
		if err := server.Serve(l); err != nil {
			fmt.Printf("err %v", err)
		}
	}()

	fmt.Println(aurora.BgMagenta("  Started!  "))

	<-done
	fmt.Println(aurora.BgRed("  Caught Signal  "))

}
