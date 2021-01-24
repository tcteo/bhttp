package bhttp

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_count",
			Help: "Count of HTTP requests.",
		},
		[]string{"handler", "method", "code"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "Histogram of latencies for HTTP requests.",
			// Buckets: []float64{.05, 0.1, .25, .5, .75, 1, 2, 5, 20, 60},
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"handler", "method", "code"},
	)
	responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Histogram of response size for HTTP requests.",
			Buckets: prometheus.ExponentialBuckets(100, 3.16227766017, 10),
		},
		[]string{"handler", "method", "code"},
	)
)

func instrumentHandler(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	var h http.Handler
	h = handler
	h = promhttp.InstrumentHandlerResponseSize(
		responseSize.MustCurryWith(prometheus.Labels{"handler": handlerName}),
		h,
	)
	h = promhttp.InstrumentHandlerDuration(
		requestDuration.MustCurryWith(prometheus.Labels{"handler": handlerName}),
		h,
	)
	h = promhttp.InstrumentHandlerCounter(
		requestCount.MustCurryWith(prometheus.Labels{"handler": handlerName}),
		h)
	return h.ServeHTTP
}

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		hf := instrumentHandler(path, next.ServeHTTP)
		hf(w, r)
	})
}

func init() {
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(responseSize)
	prometheus.MustRegister(requestCount)
}

func getEnvInt(name string) (int, error) {
	// Parse an environment variable as an int.
	s := os.Getenv(name)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return i, nil
}

func healthzHandler(rsp http.ResponseWriter, req *http.Request) {
	rsp.Header().Set("Content-Type", "text/plain")
	rsp.Write([]byte("OK"))
}

type BHttpOptions struct {
	HttpPort     int
	PromHttpPort int
}

type BHttp struct {
	Mux          *mux.Router
	promMux      *http.ServeMux
	httpPort     int
	promHttpPort int
}

func NewBHttp(opts *BHttpOptions) (*BHttp, error) {
	b := BHttp{}

	b.Mux = mux.NewRouter()
	b.Mux.Use(prometheusMiddleware)

	if opts != nil {
		// Set ports from opts if available. They can be overridden at runtime by environment variables (below).
		if opts.HttpPort != 0 {
			b.httpPort = opts.HttpPort
		}
		if opts.PromHttpPort != 0 {
			b.promHttpPort = opts.PromHttpPort
		}
	}

	envHttpPort, err := getEnvInt("HTTP_PORT")
	if err == nil {
		b.httpPort = envHttpPort
	}

	envPromHttpPort, err := getEnvInt("PROM_HTTP_PORT")
	if err == nil {
		b.promHttpPort = envPromHttpPort
	}

	b.promMux = http.NewServeMux()
	b.promMux.Handle("/metrics", promhttp.Handler())
	b.Mux.HandleFunc("/healthz", healthzHandler)

	if b.httpPort == 0 {
		return nil, fmt.Errorf("no http port specified")
	}

	return &b, nil
}

func (b *BHttp) Start() {
	// Start serving in goroutines. Does not block.
	if b.promHttpPort != 0 {
		promHttpAddr := fmt.Sprintf(":%d", b.promHttpPort)
		promHttpServer := &http.Server{
			Addr:    promHttpAddr,
			Handler: b.promMux,
		}
		log.Printf("prometheus http listening on %s", promHttpAddr)
		go func() {
			log.Fatal(promHttpServer.ListenAndServe())
		}()
	}
	if b.httpPort != 0 {
		httpAddr := fmt.Sprintf(":%d", b.httpPort)
		httpServer := &http.Server{
			Addr:    httpAddr,
			Handler: b.Mux,
		}
		log.Printf("http listening on %s", httpAddr)
		go func() {
			log.Fatal(httpServer.ListenAndServe())
		}()
	}
}
