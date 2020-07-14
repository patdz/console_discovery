package main

import (
	"context"
	"fmt"
	"github.com/patdz/consul_discovery/discovery"
	"github.com/patdz/consul_discovery/discovery/register"
	"github.com/patdz/consul_discovery/examples/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"time"
)

type server struct {
}

func (s *server) SayHello(_ context.Context, in *proto.HelloRequest) (*proto.HelloResponse, error) {
	fmt.Println("client called! 8081")
	return &proto.HelloResponse{Result: "hi," + in.Name + "!"}, nil
}

const (
	host       = "192.168.223.1"
	port       = 8082
	consulHost = "192.168.223.150"
	consulPort = 32339
)

func main() {
	// register service
	cr, err := register.NewConsulRegister(fmt.Sprintf("%s:%d", consulHost, consulPort), 15)
	if err != nil {
		log.Fatal(err)
	}
	sa, err := cr.Register(discovery.RegisterInfo{
		Host:           host,
		Port:           port,
		ServiceName:    "HelloService",
		UpdateInterval: time.Second})
	if err != nil {
		log.Fatal(err.Error())
	}

	listen, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(host), Port: port})
	if err != nil {
		fmt.Println(err.Error())
	}
	s := grpc.NewServer()

	/*
		go func() {
			time.Sleep(10 * time.Second)
			s.GracefulStop()
		}()
	*/

	proto.RegisterHelloServiceServer(s, &server{})
	reflection.Register(s)
	if err := s.Serve(listen); err != nil {
		fmt.Println("failed to serve:" + err.Error())
	}
	_ = cr.DeRegister(sa)
}
