package v2

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"

	"code.cloudfoundry.org/go-loggregator/pulseemitter"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-agent/pkg/plumbing/batching"
)

type Nexter interface {
	TryNext() (*loggregator_v2.Envelope, bool)
}

type Writer interface {
	Write(msgs []*loggregator_v2.Envelope) error
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounterMetric(name string, opts ...pulseemitter.MetricOption) pulseemitter.CounterMetric
}

type Transponder struct {
	nexter        Nexter
	writer        Writer
	tags          map[string]string
	batcher       *batching.V2EnvelopeBatcher
	batchSize     int
	batchInterval time.Duration
	droppedMetric pulseemitter.CounterMetric
	egressMetric  pulseemitter.CounterMetric
}

func NewTransponder(
	n Nexter,
	w Writer,
	tags map[string]string,
	batchSize int,
	batchInterval time.Duration,
	metricClient MetricClient,
) *Transponder {
	droppedMetric := metricClient.NewCounterMetric("dropped",
		pulseemitter.WithVersion(2, 0),
		pulseemitter.WithTags(map[string]string{"direction": "egress"}),
	)

	egressMetric := metricClient.NewCounterMetric("egress",
		pulseemitter.WithVersion(2, 0),
	)

	return &Transponder{
		nexter:        n,
		writer:        w,
		tags:          tags,
		droppedMetric: droppedMetric,
		egressMetric:  egressMetric,
		batchSize:     batchSize,
		batchInterval: batchInterval,
	}
}

func (t *Transponder) Start() {
	b := batching.NewV2EnvelopeBatcher(
		t.batchSize,
		t.batchInterval,
		batching.V2EnvelopeWriterFunc(t.write),
	)

	for {
		envelope, ok := t.nexter.TryNext()
		if !ok {
			b.Flush()
			time.Sleep(100 * time.Millisecond)
			continue
		}

		b.Write(envelope)
	}
}

func (t *Transponder) write(batch []*loggregator_v2.Envelope) {
	for _, e := range batch {
		t.addTags(e)
	}

	for _, env := range batch {
		origin, ok := env.GetTags()["origin"]
		if !ok {
			origin = ""
		}

		job, ok := env.GetTags()["job"]
		if !ok {
			job = ""
		}

		println(fmt.Sprintf("writing v2 envelope %s.%s : %v", origin, job, env))
		//TODO: add source id to tags
		//sourceId := env.SourceId
		switch x := env.Message.(type) {
		case *loggregator_v2.Envelope_Counter:
			counter := x.Counter
			name := counter.GetName()
			if _, ok := env.Tags["origin"]; ok {
				name = fmt.Sprintf("%s_%s", env.Tags["origin"], name)
			}

			name = strings.Replace(name, ".", "_", -1)
			name = strings.Replace(name, "/", "_", -1)
			name = strings.Replace(name, "-", "", -1)
			name = fmt.Sprintf("%s_total", name)

			tags := make(map[string]string)
			for key, value := range env.DeprecatedTags {
				tags[key] = value.GetText()
			}
			for key, value := range env.Tags {
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
			println(fmt.Sprintf("writting v2 counter %s with value %g and tags %v with deprecatedTags %v", name, total, env.Tags, env.DeprecatedTags))
			//metric := dto.Metric{
			//	Label: labels,
			//	Counter: &dto.Counter{
			//		Value: &total,
			//	},
			//}

			promCounter.Add(total)
		case *loggregator_v2.Envelope_Gauge:
			gaugeMetrics := x.Gauge.GetMetrics()

			for name, value := range gaugeMetrics {
				unitValue := value.GetUnit()

				if unitValue != "" {
					name = fmt.Sprintf("%s_%s", name, unitValue)
				}

				if _, ok := env.Tags["origin"]; ok {
					name = fmt.Sprintf("%s_%s", env.Tags["origin"], name)
				}

				name = strings.Replace(name, ".", "_", -1)
				name = strings.Replace(name, "/", "_", -1)
				name = strings.Replace(name, "-", "", -1)


				tags := make(map[string]string)
				for key, value := range env.DeprecatedTags {
					tags[key] = value.GetText()
				}
				for key, value := range env.Tags {
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

				total := value.GetValue()
				println(fmt.Sprintf("writting v2 gauge %s with value %g and tags %v and deprecated tags %v", name, total, env.Tags, env.DeprecatedTags))

				promGauge.Set(total)
			}
		case nil:
		default:
			continue
		}
	}

	if err := t.writer.Write(batch); err != nil {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of messages
		// dropped when failing to write to Dopplers v2 API
		t.droppedMetric.Increment(uint64(len(batch)))
		return
	}

	// metric-documentation-v2: (loggregator.metron.egress)
	// Number of messages written to Doppler's v2 API
	t.egressMetric.Increment(uint64(len(batch)))
}

func (t *Transponder) addTags(e *loggregator_v2.Envelope) {
	if e.DeprecatedTags == nil {
		e.DeprecatedTags = make(map[string]*loggregator_v2.Value)
	}

	// Move non-deprecated tags to deprecated tags. This is required
	// for backwards compatibility purposes and should be removed once
	// deprecated tags are fully removed.
	for k, v := range e.GetTags() {
		e.DeprecatedTags[k] = &loggregator_v2.Value{
			Data: &loggregator_v2.Value_Text{
				Text: v,
			},
		}
	}

	for k, v := range t.tags {
		if _, ok := e.DeprecatedTags[k]; !ok {
			e.DeprecatedTags[k] = &loggregator_v2.Value{
				Data: &loggregator_v2.Value_Text{
					Text: v,
				},
			}
		}
	}
}
