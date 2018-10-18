package cdn

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"particles/api"
	"particles/cache"
	"particles/util"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	errCacheInit = errors.New("error initializing cache")
	errAPIInit   = errors.New("error initializing API")
)

// CDN represents the CDN ojbect
type CDN struct {
	api          *api.API
	cache        cache.Cache
	httpServer   *http.Server
	httpEnabled  bool
	httpsServer  *http.Server
	httpsEnabled bool
	httpMux      *http.ServeMux
	endpoints    map[string]endpoint
}

type endpoint struct {
	IP    string
	Port  int
	Proto string
}

// NewCDN returns a new CDN object
func NewCDN(conf Conf) (*CDN, error) {
	// Cache
	c, err := cache.NewCache(conf.Cache)
	if err != nil {
		return nil, errCacheInit
	}

	// API
	a, err := api.NewAPI(conf.API, c)
	if err != nil {
		return nil, errAPIInit
	}

	mux := http.NewServeMux()

	// HTTP
	lHTTPAddr := fmt.Sprintf("%s:%d", conf.HTTP.Address, conf.HTTP.Port)
	s := &http.Server{
		Addr:           lHTTPAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// HTTPS
	cfg := &tls.Config{}
	for _, b := range conf.HTTPS.Backends {
		cert, err := tls.LoadX509KeyPair(b.CertFile, b.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("certificate error for %s (%s)", b.Name, b.Domain)
		}
		cfg.Certificates = append(cfg.Certificates, cert)
	}
	cfg.BuildNameToCertificate()

	lHTTPSAddr := fmt.Sprintf("%s:%d", conf.HTTPS.Address, conf.HTTPS.Port)
	ss := &http.Server{
		Addr:           lHTTPSAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      cfg,
	}

	// populate endpoints
	eps := make(map[string]endpoint, 0)
	for _, e := range conf.HTTP.Backends {
		port := conf.HTTP.Port
		if e.Port > 0 {
			port = e.Port
		}
		eps[e.Domain] = endpoint{IP: e.IP, Port: port, Proto: "http"}
	}
	for _, e := range conf.HTTPS.Backends {
		port := conf.HTTPS.Port
		if e.Port > 0 {
			port = e.Port
		}
		eps[e.Domain] = endpoint{IP: e.IP, Port: port, Proto: "https"}
	}

	return &CDN{api: a, cache: c, httpServer: s, httpEnabled: len(conf.HTTP.Backends) > 0, httpsServer: ss, httpsEnabled: len(conf.HTTPS.Backends) > 0, httpMux: mux, endpoints: eps}, nil
}

// Start starts the CDN by starting the HTTP/HTTPS endpoint and API. It returns a channel which can be
// used to understand when an error occurs in one of the handlers
func (c *CDN) Start() <-chan struct{} {
	exit := make(chan struct{})
	// The server where the CDN runs should know what the real IP address for a website is
	// For now we override it forcibly
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 10 * time.Second,
		DualStack: true,
	}
	http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		// the address always includes the port, so we split
		addrParts := strings.Split(addr, ":")
		host := addrParts[0]
		port := addrParts[1]

		// check we manage that endpoint
		e, ok := c.endpoints[host]
		if !ok {
			return nil, fmt.Errorf("unhandled endpoint")
		}

		// override IP and/or port if defined
		if e.IP != "" {
			host = e.IP
		}
		if e.Port > 0 {
			port = strconv.Itoa(e.Port)
		}

		addr = fmt.Sprintf("%s:%s", host, port)
		return dialer.DialContext(ctx, network, addr)
	}

	c.httpMux.Handle("/", util.WithLogging(c.httpHandler))

	// API server
	go func() {
		err := c.api.Start()
		if err != nil {
			logrus.Errorf("API server exited: %s", err)
			close(exit)
		}
	}()

	// HTTP server
	if c.httpEnabled {
		go func() {
			logrus.Infof("Starting listerner on %s (HTTP)", c.httpServer.Addr)
			err := c.httpServer.ListenAndServe()
			if err != nil {
				logrus.Errorf("HTTP server exited: %s", err)
				close(exit)
			}
		}()
	}

	// HTTPS server
	if c.httpsEnabled {
		go func() {
			logrus.Infof("Starting listerner on %s (HTTPS)", c.httpsServer.Addr)
			err := c.httpsServer.ListenAndServeTLS("", "")
			if err != nil {
				logrus.Errorf("HTTPS server exited: %s", err)
				close(exit)
			}
		}()
	}

	return exit
}

// Shutdown terminates the CDN in a clean way
func (c *CDN) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := c.httpServer.Shutdown(ctx)
	if err != nil {
		return err
	}

	err = c.httpsServer.Shutdown(ctx)
	if err != nil {
		return err
	}

	err = c.api.Shutdown(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (c *CDN) httpHandler(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer requestDuration.Observe(time.Since(start).Seconds())

	host := req.Host
	h, _, err := net.SplitHostPort(req.Host)
	if err == nil {
		host = h
	}
	port := c.endpoints[host].Port
	proto := c.endpoints[host].Proto
	backend := fmt.Sprintf("%s://%s:%d", proto, host, port)
	fr := fmt.Sprintf("%s%s", backend, req.URL.Path)

	reqURL := req.URL.String()

	// Do a lookup and if present return directly without making a HTTP request
	content, found, err := c.cache.Lookup(reqURL)
	if err != nil {
		logrus.Debugf("error while looking up %s: %s", fr, err)
	}
	if found {
		logrus.Debugf("cache hit: %s (%s)", fr, content.ContentType)
		requestsMetric.WithLabelValues(strconv.Itoa(http.StatusOK), "error").Inc()
		for k, v := range content.Headers() {
			w.Header().Set(k, string(v))
		}
		w.WriteHeader(http.StatusOK)
		w.Write(content.Content())
		return
	}

	logrus.Debugf("cache miss: %s", fr)
	// creating a client to send the full request
	client := &http.Client{}
	r, err := http.NewRequest(req.Method, fr, req.Body)
	if err != nil {
		logrus.Errorf("error creating a new proxy request: %s", err)
		requestsMetric.WithLabelValues(strconv.Itoa(http.StatusBadRequest), "error").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// propagating all headers to the backend request
	for k, v := range req.Header {
		logrus.Debugf("propagating headers to backend %s: %s", k, v[0])
		r.Header.Add(k, v[0])
	}

	// execute the request to the backend
	resp, err := client.Do(r)
	if err != nil {
		logrus.Errorf("error proxying request: %s", err)
		requestsMetric.WithLabelValues(strconv.Itoa(http.StatusBadRequest), "error").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	// read the response body
	rb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("error reading response body: %s", err)
		requestsMetric.WithLabelValues(strconv.Itoa(http.StatusInternalServerError), "error").Inc()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" {
		logrus.Debugf("[%s] Content-type: %s", fr, ct)
		if c.cache.IsCachableContentType(ct) {
			logrus.Debugf("content type can be cached")

			cc := resp.Header.Get("Cache-Control")
			ccParts := strings.Split(strings.TrimSpace(cc), ",")
			var ttl int
			cachable := false

			for _, s := range ccParts {
				// handle max age
				if strings.HasPrefix(s, "max-age") {
					maParts := strings.Split(s, "=")
					newTTL, err := strconv.Atoi(maParts[1])
					if err != nil {
						logrus.Debugf("invalid TTL: %s", err)
					} else {
						ttl = newTTL
					}
				}

				// handle caching headers: cache only if we are allowed
				if strings.ToLower(s) == "public" {
					logrus.Debugf("Cache-Control: %s, it's ok to cache", s)
					cachable = true
				}
			}

			if cachable {
				logrus.Infof("storing a new object in cache: %s (%s)", fr, ct)

				// we also want to store the headers
				h := make(map[string]string, 0)
				for k, v := range resp.Header {
					h[k] = v[0]
				}
				co := cache.NewContentObject(rb, ct, h, ttl)
				// avoid delaying the response to the user because
				// the object is being stored, hence use a go routine
				// Prefer serving the user as fast as possible rather than checking if
				// there was an error storing the object. It will be picked up via metrics/logs.
				go c.cache.Store(reqURL, co)
			}
		}
	}

	// forward response headers
	for k, v := range resp.Header {
		w.Header().Set(k, v[0])
	}

	requestsMetric.WithLabelValues(strconv.Itoa(resp.StatusCode), "success").Inc()
	w.WriteHeader(resp.StatusCode)
	w.Write(rb)
}
