package listeners

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wind-c/comqtt/server/listeners/auth"
	"github.com/wind-c/comqtt/server/system"
)

// HTTPStats is a listener for presenting the server $SYS stats on a JSON http endpoint.
type HTTPStats struct {
	sync.RWMutex
	id       string       // the internal id of the listener.
	address  string       // the network address to bind to.
	config   *Config      // configuration values for the listener.
	system   *system.Info // pointers to the server data.
	listen   *http.Server // the http server.
	end      uint32       // ensure the close methods are only called once.}
	handlers map[string]func(http.ResponseWriter, *http.Request)
}

func NewH(id, address string, handlers map[string]func(http.ResponseWriter, *http.Request)) *HTTPStats {
	return &HTTPStats{
		id:      id,
		address: address,
		config: &Config{
			Auth: new(auth.Allow),
		},
		handlers: handlers,
	}
}

// NewHTTPStats initialises and returns a new HTTP listener, listening on an address.
func NewHTTPStats(id, address string) *HTTPStats {
	return NewH(id, address, nil)
}

// SetConfig sets the configuration values for the listener config.
func (l *HTTPStats) SetConfig(config *Config) {
	l.Lock()
	if config != nil {
		l.config = config

		// If a config has been passed without an auth controller,
		// it may be a mistake, so disallow all traffic.
		if l.config.Auth == nil {
			l.config.Auth = new(auth.Disallow)
		}
	}

	l.Unlock()
}

// ID returns the id of the listener.
func (l *HTTPStats) ID() string {
	l.RLock()
	id := l.id
	l.RUnlock()
	return id
}

type metrics struct {
	Count *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		Count: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_counter",
			Help: "测试的数据",
		}, []string{"test1", "test2", "test3"}),
	}
	reg.MustRegister(m.Count)
	return m
}

// Listen starts listening on the listener's network address.
func (l *HTTPStats) Listen(s *system.Info) error {
	l.system = s
	mux := http.NewServeMux()

	reg := prometheus.NewRegistry()

	// Add go runtime metrics and process collectors.
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	m := NewMetrics(reg)
	mux.HandleFunc("/", l.jsonHandler)
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	for path, handler := range l.handlers {
		mux.HandleFunc(path, handler)
	}

	l.listen = &http.Server{
		Addr:    l.address,
		Handler: mux,
	}

	go func() {
		for {

			m.Count.WithLabelValues("test1", "test2", "test3").Add(20)
			time.Sleep(time.Duration(10) * time.Millisecond)
		}
	}()
	// The following logic is deprecated in favour of passing through the tls.Config
	// value directly, however it remains in order to provide backwards compatibility.
	// It will be removed someday, so use the preferred method (l.config.TLSConfig).
	if l.config.TLS != nil && len(l.config.TLS.Certificate) > 0 && len(l.config.TLS.PrivateKey) > 0 {
		cert, err := tls.X509KeyPair(l.config.TLS.Certificate, l.config.TLS.PrivateKey)
		if err != nil {
			return err
		}

		l.listen.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	} else {
		l.listen.TLSConfig = l.config.TLSConfig
	}

	return nil
}

// Serve starts listening for new connections and serving responses.
func (l *HTTPStats) Serve(establish EstablishFunc) {
	if l.listen.TLSConfig != nil {
		l.listen.ListenAndServeTLS("", "")
	} else {
		l.listen.ListenAndServe()
	}
}

// Close closes the listener and any client connections.
func (l *HTTPStats) Close(closeClients CloseFunc) {
	l.Lock()
	defer l.Unlock()

	if atomic.CompareAndSwapUint32(&l.end, 0, 1) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l.listen.Shutdown(ctx)
	}

	closeClients(l.id)
}

// jsonHandler is an HTTP handler which outputs the $SYS stats as JSON.
func (l *HTTPStats) jsonHandler(w http.ResponseWriter, req *http.Request) {
	info, err := json.MarshalIndent(l.system, "", "\t")
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	w.Write(info)
}
