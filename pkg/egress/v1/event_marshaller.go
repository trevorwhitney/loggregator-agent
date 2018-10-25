package v1

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"strings"
	"sync"

	"code.cloudfoundry.org/go-loggregator/pulseemitter"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

//go:generate hel --type BatchChainByteWriter --output mock_writer_test.go

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounterMetric(name string, opts ...pulseemitter.MetricOption) pulseemitter.CounterMetric
}

type BatchChainByteWriter interface {
	Write(message []byte) (err error)
}

type EventMarshaller struct {
	egressCounter pulseemitter.CounterMetric
	byteWriter    BatchChainByteWriter
	bwLock        sync.RWMutex
}

func NewMarshaller(mc MetricClient) *EventMarshaller {
	return &EventMarshaller{
		egressCounter: mc.NewCounterMetric("egress"),
	}
}

func (m *EventMarshaller) SetWriter(byteWriter BatchChainByteWriter) {
	m.bwLock.Lock()
	defer m.bwLock.Unlock()
	m.byteWriter = byteWriter
}

func (m *EventMarshaller) writer() BatchChainByteWriter {
	m.bwLock.RLock()
	defer m.bwLock.RUnlock()
	return m.byteWriter
}

func (m *EventMarshaller) Write(envelope *events.Envelope) {
	switch envelope.GetEventType() {
	case events.Envelope_CounterEvent:
		counter := envelope.GetCounterEvent()
		name := counter.GetName()
		name = strings.Replace(name, ".", "_", -1)
		name = strings.Replace(name, "/", "_", -1)
		name = strings.Replace(name, "-", "_", -1)

		tags := make(map[string]string)
		for key, value := range envelope.Tags {
			tags[key] = value
		}

		counterOpts := prometheus.CounterOpts{
			Name:        name,
			ConstLabels: tags,
		}

		promCounter := prometheus.NewCounter(counterOpts)
		err := prometheus.Register(promCounter)
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if col, ok := are.ExistingCollector.(prometheus.Counter); ok {
				promCounter = col
			}
		}

		total := float64(counter.GetTotal())
		println(fmt.Sprintf("writting V1 counter %s with value %g and tags %v", name, total, envelope.Tags))
		promCounter.Add(total)
	case events.Envelope_ValueMetric:
		gaugeMetric := envelope.GetValueMetric()

		name := gaugeMetric.GetName()
		value := gaugeMetric.GetValue()

		name = strings.Replace(name, ".", "_", -1)
		name = strings.Replace(name, "/", "_", -1)
		name = strings.Replace(name, "-", "_", -1)

		tags := make(map[string]string)
		for key, value := range envelope.Tags {
			tags[key] = value
		}

		gaugeOpts := prometheus.GaugeOpts{
			Name:        name,
			ConstLabels: tags,
		}

		promGauge := prometheus.NewGauge(gaugeOpts)
		err := prometheus.Register(promGauge)
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if col, ok := are.ExistingCollector.(prometheus.Gauge); ok {
				promGauge = col
			}
		}

		println(fmt.Sprintf("writting V1 gauge %s with value %g and tags %v", name, value, envelope.Tags))
		promGauge.Set(value)
	}



	writer := m.writer()
	if writer == nil {
		log.Print("EventMarshaller: Write called while byteWriter is nil")
		return
	}
	println("got v1 message EventMashaller")
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		log.Printf("marshalling error: %v", err)
		return
	}

	writer.Write(envelopeBytes)
	m.egressCounter.Increment(1)
}
