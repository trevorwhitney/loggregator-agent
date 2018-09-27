// This file was generated by github.com/nelsam/hel.  Do not
// edit this code by hand unless you *really* know what you're
// doing.  Expect any changes made manually to be overwritten
// the next time hel regenerates this file.

package agent_test

import (
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing"
	"golang.org/x/net/context"
)

type mockDopplerIngestorServerV1 struct {
	PusherCalled chan bool
	PusherInput  struct {
		Arg0 chan plumbing.DopplerIngestor_PusherServer
	}
	PusherOutput struct {
		Ret0 chan error
	}
}

func newMockDopplerIngestorServerV1() *mockDopplerIngestorServerV1 {
	m := &mockDopplerIngestorServerV1{}
	m.PusherCalled = make(chan bool, 100)
	m.PusherInput.Arg0 = make(chan plumbing.DopplerIngestor_PusherServer, 100)
	m.PusherOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockDopplerIngestorServerV1) Pusher(arg0 plumbing.DopplerIngestor_PusherServer) error {
	m.PusherCalled <- true
	m.PusherInput.Arg0 <- arg0
	return <-m.PusherOutput.Ret0
}

type mockIngressServerV2 struct {
	SendCalled chan bool
	SendInput  struct {
		Arg0 chan context.Context
		Arg1 chan *loggregator_v2.EnvelopeBatch
	}
	SendOutput struct {
		Ret0 chan *loggregator_v2.SendResponse
		Ret1 chan error
	}

	SenderCalled chan bool
	SenderInput  struct {
		Arg0 chan loggregator_v2.Ingress_SenderServer
	}
	SenderOutput struct {
		Ret0 chan error
	}

	BatchSenderCalled chan bool
	BatchSenderInput  struct {
		Arg0 chan loggregator_v2.Ingress_BatchSenderServer
	}
	BatchSenderOutput struct {
		Ret0 chan error
	}
}

func newMockIngressServerV2() *mockIngressServerV2 {
	m := &mockIngressServerV2{}
	m.SendCalled = make(chan bool, 100)
	m.SendInput.Arg0 = make(chan context.Context, 100)
	m.SendInput.Arg1 = make(chan *loggregator_v2.EnvelopeBatch, 100)
	m.SendOutput.Ret0 = make(chan *loggregator_v2.SendResponse, 100)
	m.SendOutput.Ret1 = make(chan error, 100)

	m.SenderCalled = make(chan bool, 100)
	m.SenderInput.Arg0 = make(chan loggregator_v2.Ingress_SenderServer, 100)
	m.SenderOutput.Ret0 = make(chan error, 100)

	m.BatchSenderCalled = make(chan bool, 100)
	m.BatchSenderInput.Arg0 = make(chan loggregator_v2.Ingress_BatchSenderServer, 100)
	m.BatchSenderOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockIngressServerV2) Send(arg0 context.Context, arg1 *loggregator_v2.EnvelopeBatch) (*loggregator_v2.SendResponse, error) {
	m.SendCalled <- true
	m.SendInput.Arg0 <- arg0
	m.SendInput.Arg1 <- arg1
	return <-m.SendOutput.Ret0, <-m.SendOutput.Ret1
}
func (m *mockIngressServerV2) Sender(arg0 loggregator_v2.Ingress_SenderServer) error {
	m.SenderCalled <- true
	m.SenderInput.Arg0 <- arg0
	return <-m.SenderOutput.Ret0
}
func (m *mockIngressServerV2) BatchSender(arg0 loggregator_v2.Ingress_BatchSenderServer) error {
	m.BatchSenderCalled <- true
	m.BatchSenderInput.Arg0 <- arg0
	return <-m.BatchSenderOutput.Ret0
}
