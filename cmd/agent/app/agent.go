package app

import (
	"expvar"
	"fmt"
	"log"
	"net"
	"net/http"

	"code.cloudfoundry.org/loggregator-agent/pkg/metrics"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
)

type Agent struct {
	config *Config
	lookup func(string) ([]net.IP, error)
}

// AgentOption configures agent options.
type AgentOption func(*Agent)

// WithLookup allows the default DNS resolver to be changed.
func WithLookup(l func(string) ([]net.IP, error)) func(*Agent) {
	return func(a *Agent) {
		a.lookup = l
	}
}

func NewAgent(
	c *Config,
	opts ...AgentOption,
) *Agent {
	a := &Agent{
		config: c,
		lookup: net.LookupIP,
	}

	for _, o := range opts {
		o(a)
	}

	return a
}

func (a *Agent) Start() {
	clientCreds, err := plumbing.NewClientCredentials(
		a.config.GRPC.CertFile,
		a.config.GRPC.KeyFile,
		a.config.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for client: %s", err)
	}

	var opts []plumbing.ConfigOption
	if len(a.config.GRPC.CipherSuites) > 0 {
		opts = append(opts, plumbing.WithCipherSuites(a.config.GRPC.CipherSuites))
	}

	serverCreds, err := plumbing.NewServerCredentials(
		a.config.GRPC.CertFile,
		a.config.GRPC.KeyFile,
		a.config.GRPC.CAFile,
		opts...,
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	metricClient := metrics.New(expvar.NewMap("Agent"))

	appV1 := NewV1App(a.config, clientCreds, metricClient)
	go appV1.Start()

	appV2 := NewV2App(a.config, clientCreds, serverCreds, metricClient)
	go appV2.Start()

	go a.runHealthEndpoint()
}

func (a *Agent) runHealthEndpoint() {
	addr := fmt.Sprintf("127.0.0.1:%d", a.config.HealthEndpointPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Unable to setup Health endpoint (%s): %s", addr, err)
	}
	log.Printf("health bound to: %s", lis.Addr())

	http.Serve(lis, nil)
}
