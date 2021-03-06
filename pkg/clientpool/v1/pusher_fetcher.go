package v1

import (
	"context"
	"fmt"
	"io"
	"log"

	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"

	"google.golang.org/grpc"
)

type MetricClient interface {
	NewSumGauge(name string) func(float64)
}

type PusherFetcher struct {
	opts               []grpc.DialOption
	dopplerConnections func(float64)
	dopplerV1Streams   func(float64)
}

func NewPusherFetcher(mc MetricClient, opts ...grpc.DialOption) *PusherFetcher {
	return &PusherFetcher{
		opts:               opts,
		dopplerConnections: mc.NewSumGauge("DopplerConnections"),
		dopplerV1Streams:   mc.NewSumGauge("DopplerV1Streams"),
	}
}

func (p *PusherFetcher) Fetch(addr string) (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}
	p.dopplerConnections(1)

	client := plumbing.NewDopplerIngestorClient(conn)

	pusher, err := client.Pusher(context.Background())
	if err != nil {
		p.dopplerConnections(-1)
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", addr, err)
	}
	p.dopplerV1Streams(1)

	log.Printf("successfully established a stream to doppler %s", addr)

	closer := &decrementingCloser{
		closer:             conn,
		dopplerConnections: p.dopplerConnections,
		dopplerV1Streams:   p.dopplerV1Streams,
	}
	return closer, pusher, err
}

type decrementingCloser struct {
	closer             io.Closer
	dopplerConnections func(float64)
	dopplerV1Streams   func(float64)
}

func (d *decrementingCloser) Close() error {
	d.dopplerConnections(-1)
	d.dopplerV1Streams(-1)

	return d.closer.Close()
}
