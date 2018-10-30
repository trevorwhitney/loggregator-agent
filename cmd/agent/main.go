package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"code.cloudfoundry.org/loggregator-agent/cmd/agent/app"
	"google.golang.org/grpc/grpclog"
)

var addr = flag.String("listen-address", ":8888", "The address to listen on for HTTP requests.")

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	rand.Seed(time.Now().UnixNano())
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

	config, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("Unable to parse config: %s", err)
	}

	a := app.NewAgent(config)
	go a.Start()

	certificate, err := tls.LoadX509KeyPair(
		config.GRPC.CertFile,
		config.GRPC.KeyFile,
	)
	if err != nil {
		panic(err)
	}

	certPool := x509.NewCertPool()
	caCert, err := ioutil.ReadFile(config.GRPC.CAFile)
	if err != nil {
		panic(err)
	}

	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		panic(err)
	}

	tlsConfig := &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}

	tlsConfig.BuildNameToCertificate()

	scrapeIntervalSeconds := int64(15)
	startupTime := time.Now().Unix()

	numberOfScrapeIntervalsCounterOpts := prometheus.CounterOpts{
		Name:        "observability_metron_scrape_intervals_total",
		ConstLabels: map[string]string{
			"ip": config.IP,
			"index": config.Index,
			"job": config.Job,
			"deployment": config.Deployment,
		},
	}

	numberOfScrapeIntervalsCounter := prometheus.NewCounter(numberOfScrapeIntervalsCounterOpts)
	err = prometheus.Register(numberOfScrapeIntervalsCounter)
	if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
		if col, ok := are.ExistingCollector.(prometheus.Counter); ok {
			numberOfScrapeIntervalsCounter = col
		}
	}

	actualScrapedIntervalsCounterOpts := prometheus.CounterOpts{
		Name:        "observability_metron_actual_scrapes_total",
		ConstLabels: map[string]string{
			"ip": config.IP,
			"index": config.Index,
			"job": config.Job,
			"deployment": config.Deployment,
		},
	}

	actualScrapedIntervalsCounter := prometheus.NewCounter(actualScrapedIntervalsCounterOpts)
	err = prometheus.Register(actualScrapedIntervalsCounter)
	if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
		if col, ok := are.ExistingCollector.(prometheus.Counter); ok {
			actualScrapedIntervalsCounter = col
		}
	}

	counterOpts := prometheus.CounterOpts{
		Name:        "observability_metron_scrape_total",
		ConstLabels: map[string]string{
			"ip": config.IP,
			"index": config.Index,
			"job": config.Job,
			"deployment": config.Deployment,
		},
	}

	promCounter := prometheus.NewCounter(counterOpts)
	err = prometheus.Register(promCounter)
	if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
		if col, ok := are.ExistingCollector.(prometheus.Counter); ok {
			promCounter = col
		}
	}

	scrapeTimeCounterOpts := prometheus.CounterOpts{
		Name:        "observability_metron_scrape_last_occurred",
		ConstLabels: map[string]string{
			"ip": config.IP,
			"index": config.Index,
			"job": config.Job,
			"deployment": config.Deployment,
		},
	}

	scrapeTimeCounter := prometheus.NewCounter(scrapeTimeCounterOpts)
	err = prometheus.Register(scrapeTimeCounter)
	if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
		if col, ok := are.ExistingCollector.(prometheus.Counter); ok {
			scrapeTimeCounter = col
		}
	}

	scrapeTimeGaugeOpts := prometheus.GaugeOpts{
		Name:        "observability_metron_scrape_last_occurred_gauge",
		ConstLabels: map[string]string{
			"ip": config.IP,
			"index": config.Index,
			"job": config.Job,
			"deployment": config.Deployment,
		},
	}

	scrapeTimeGauge := prometheus.NewGauge(scrapeTimeGaugeOpts)
	err = prometheus.Register(scrapeTimeGauge)
	if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
		if col, ok := are.ExistingCollector.(prometheus.Gauge); ok {
			scrapeTimeGauge = col
		}
	}
	lastScrapeTime := int64(0)
	scrapeHandlerWithMetrics := func(response http.ResponseWriter, request *http.Request) {
		intervals := math.Floor(float64((time.Now().Unix() - startupTime)) / float64(scrapeIntervalSeconds))
		numberOfScrapeIntervalsCounter.Add(float64(intervals) + float64(1)) //Intervals should be 1-indexed to match scrapes
		currentScrapeInterval := time.Now().Truncate(time.Duration(scrapeIntervalSeconds) * time.Second).Unix()

		if lastScrapeTime != currentScrapeInterval {
			actualScrapedIntervalsCounter.Inc()
			lastScrapeTime = currentScrapeInterval
		}

		promCounter.Inc()
		currentTime := time.Now().Unix()
		scrapeTimeCounter.Add(float64(currentTime))
		scrapeTimeGauge.Set(float64(currentTime))
		promhttp.Handler().ServeHTTP(response, request)
	}

	http.HandleFunc("/metrics", scrapeHandlerWithMetrics)
	httpServer := &http.Server{
		Addr:      ":8888",
		TLSConfig: tlsConfig,
	}
	go log.Fatal(httpServer.ListenAndServeTLS(config.GRPC.CertFile, config.GRPC.KeyFile))

	runPProf(config.PProfPort)
}

func runPProf(port uint32) {
	addr := fmt.Sprintf("localhost:%d", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Panicf("Error creating pprof listener: %s", err)
	}

	log.Printf("pprof bound to: %s", lis.Addr())
	err = http.Serve(lis, nil)
	if err != nil {
		log.Panicf("Error starting pprof server: %s", err)
	}
}
