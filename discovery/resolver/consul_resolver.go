package resolver

import (
	"context"
	"fmt"
	consulapi "github.com/hashicorp/consul/api"
	"google.golang.org/grpc/resolver"
	"log"
	"sync"
	"time"
)

type consulBuilder struct {
	schema       string
	consulClient *consulapi.Client
}

func NewConsulBuilder(address, schema string) (resolver.Builder, error) {
	config := consulapi.DefaultConfig()
	config.Address = address
	client, err := consulapi.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &consulBuilder{schema: schema, consulClient: client}, nil
}

func (cb *consulBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	adds, err := cb.resolve(target.Endpoint)
	if err != nil {
		return nil, err
	}
	cc.UpdateState(resolver.State{
		Addresses: adds,
	})
	consulResolver := NewConsulResolver(&cc, cb, opts)
	consulResolver.wg.Add(1)
	go consulResolver.watcher(target.Endpoint)

	return consulResolver, nil
}

func (cb consulBuilder) resolve(serverName string) ([]resolver.Address, error) {
	serviceEntries, _, err := cb.consulClient.Health().Service(serverName, "", true, &consulapi.QueryOptions{})
	if err != nil {
		return nil, err
	}

	adds := make([]resolver.Address, 0)
	for _, serviceEntry := range serviceEntries {
		address := resolver.Address{Addr: fmt.Sprintf("%s:%d", serviceEntry.Service.Address, serviceEntry.Service.Port)}
		adds = append(adds, address)
	}
	return adds, nil
}

func (cb *consulBuilder) Scheme() string {
	return cb.schema
}

type consulResolver struct {
	clientConn    *resolver.ClientConn
	consulBuilder *consulBuilder
	t             *time.Ticker
	wg            sync.WaitGroup
	rn            chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewConsulResolver(cc *resolver.ClientConn, cb *consulBuilder, _ resolver.BuildOptions) *consulResolver {
	ctx, cancel := context.WithCancel(context.Background())
	return &consulResolver{
		clientConn:    cc,
		consulBuilder: cb,
		t:             time.NewTicker(time.Second),
		rn:            make(chan struct{}, 10),
		ctx:           ctx,
		cancel:        cancel}
}

func (cr *consulResolver) watcher(serverName string) {
	defer cr.wg.Done()
	for {
		select {
		case <-cr.ctx.Done():
			return
		case <-cr.rn:
		case <-cr.t.C:
		}
		adds, err := cr.consulBuilder.resolve(serverName)
		if err != nil {
			log.Printf("query service entries [%s] error: %v", serverName, err.Error())
		}
		(*cr.clientConn).UpdateState(resolver.State{
			Addresses: adds,
		})
	}
}

func (cr *consulResolver) Scheme() string {
	return cr.consulBuilder.Scheme()
}

func (cr *consulResolver) ResolveNow(_ resolver.ResolveNowOptions) {
	select {
	case cr.rn <- struct{}{}:
	default:
	}
}

func (cr *consulResolver) Close() {
	cr.cancel()
	cr.wg.Wait()
	cr.t.Stop()
}

func GenerateAndRegisterConsulResolver(address, schema string) error {
	builder, err := NewConsulBuilder(address, schema)
	if err != nil {
		return nil
	}
	resolver.Register(builder)
	return nil
}
