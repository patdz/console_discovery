package main

import (
	"context"
	"fmt"
	"github.com/patdz/consul_discovery/discovery/resolver"
	"github.com/patdz/consul_discovery/examples/proto"
	"google.golang.org/grpc"
	"log"
	"time"
)

const (
	consulHost = "192.168.223.150"
	consulPort = 32339
	scheme     = "consul_demo"
)

func main() {
	//
	err := resolver.GenerateAndRegisterConsulResolver(fmt.Sprintf("%s:%d", consulHost, consulPort), scheme)
	if err != nil {
		log.Fatal("init consul resovler err", err.Error())
	}

	// Set up a connection to the server.
	conn, err := grpc.Dial(fmt.Sprintf("%s:///HelloService", scheme), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	c := proto.NewHelloServiceClient(conn)

	// Contact the server and print out its response.
	name := "user1"

	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		r, err := c.SayHello(ctx, &proto.HelloRequest{Name: name})
		if err != nil {
			log.Println("could not greet: %v", err)

		} else {
			log.Printf("Hello: %s", r.Result)
		}
		time.Sleep(time.Second)
	}

}
