package register

import (
	"context"
	"fmt"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/patdz/consul_discovery/discovery"
	"log"
	"sync"
	"time"
)

type ServerEntry struct {
	sync.WaitGroup
	cancel   context.CancelFunc
	serverId string
}

type ConsulRegister struct {
	client *consulapi.Client
	ttl    int
}

func NewConsulRegister(address string, ttl int) (*ConsulRegister, error) {
	config := consulapi.DefaultConfig()
	config.Address = address
	client, err := consulapi.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &ConsulRegister{client: client, ttl: ttl}, nil
}

func (cr *ConsulRegister) Register(info discovery.RegisterInfo) (*ServerEntry, error) {
	serviceId := generateServiceId(info.ServiceName, info.Host, info.Port)

	// initial register
	reg := &consulapi.AgentServiceRegistration{
		ID:      serviceId,
		Name:    info.ServiceName,
		Tags:    []string{info.ServiceName},
		Port:    info.Port,
		Address: info.Host,
	}

	if err := cr.client.Agent().ServiceRegister(reg); err != nil {
		return nil, err
	}

	// initial register service check
	check := consulapi.AgentServiceCheck{TTL: fmt.Sprintf("%ds", cr.ttl), Status: consulapi.HealthPassing}
	err := cr.client.Agent().CheckRegister(
		&consulapi.AgentCheckRegistration{
			ID:                serviceId,
			Name:              info.ServiceName,
			ServiceID:         serviceId,
			AgentServiceCheck: check})
	if err != nil {
		return nil, fmt.Errorf("initial register service check to consul error: %s", err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	sa := &ServerEntry{cancel: cancel, serverId: serviceId}
	sa.Add(1)
	go func() {
		defer sa.Done()
		ticker := time.NewTicker(info.UpdateInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err = cr.client.Agent().UpdateTTL(serviceId, "", check.Status)
				if err != nil {
					log.Println("update ttl of service error: ", err.Error())
				}
			}
		}
	}()

	return sa, nil
}

func (cr *ConsulRegister) DeRegister(sa *ServerEntry) error {

	err := cr.client.Agent().ServiceDeregister(sa.serverId)
	if err != nil {
		log.Println("deregister service error: ", err.Error())
	} else {
		log.Println("de registered service from consul server.")
	}

	err = cr.client.Agent().CheckDeregister(sa.serverId)
	if err != nil {
		log.Println("LearnGrpc: deregister check error: ", err.Error())
		return err
	}

	sa.cancel()
	sa.Wait()

	return nil
}

func generateServiceId(name, host string, port int) string {
	return fmt.Sprintf("%s-%s-%d", name, host, port)
}
