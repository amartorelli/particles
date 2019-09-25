package cdn

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/amartorelli/particles/pkg/api"
	"github.com/amartorelli/particles/pkg/cache"
	"github.com/amartorelli/particles/pkg/util"
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
	httpClient   *http.Client
}

// endpoint is a structure to represent an endpoint handled by the Particles
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

	return &CDN{
		api:          a,
		cache:        c,
		httpServer:   s,
		httpEnabled:  len(conf.HTTP.Backends) > 0,
		httpsServer:  ss,
		httpsEnabled: len(conf.HTTPS.Backends) > 0,
		httpMux:      mux,
		endpoints:    eps,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}, nil
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
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

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

	c.httpMux.Handle("/", util.HandlerWithLogging(c.httpHandler))

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

type cacheItemInfo struct {
	ContentType string
	MaxAge      int
}

// isCachable checks if the Cache-Control header specifies the resource as public
func (c *CDN) isCachable(headers http.Header) (bool, cacheItemInfo) {
	cii := cacheItemInfo{}
	// handle Content-Type header to cache if possible
	ct := headers.Get("Content-Type")
	if ct == "" {
		return false, cii
	}

	if !c.cache.IsCachableContentType(ct) {
		logrus.Debugf("content type cannot be cached")
		return false, cii
	}
	cii.ContentType = ct

	cc := headers.Get("Cache-Control")
	if cc == "" {
		return false, cii
	}

	ccParts := strings.Split(strings.TrimSpace(cc), ",")
	cachable := false

	for _, s := range ccParts {
		trimS := strings.TrimSpace(s)
		if strings.ToLower(trimS) == "public" {
			logrus.Debugf("Cache-Control: %s, it's ok to cache", trimS)
			cachable = true
		}

		if strings.HasPrefix(trimS, "max-age") {
			maParts := strings.Split(trimS, "=")
			newTTL, err := strconv.Atoi(maParts[1])
			if err != nil {
				logrus.Debugf("invalid TTL: %s", err)
			} else {
				cii.MaxAge = newTTL
			}
		}
	}

	logrus.Debugf("content type can be cached")
	return cachable, cii
}

// respHeadersToMap converts headers in  respons to a map of strings
func respHeadersToMap(resp *http.Response) map[string]string {
	h := make(map[string]string, 0)
	for k, v := range resp.Header {
		h[k] = v[0]
	}
	return h
}

// cleanHeadersMap removes from a map of headers all the headers that shouldn't be cached
func cleanHeadersMap(hh map[string]string) map[string]string {
	// TODO: this function should strip out headers that shouldn't be cached
	return hh
}

// validate implements the validation by sending a request with the If-Modified-Since header
func (c *CDN) validate(req *http.Request) (validated bool, resp *http.Response, err error) {
	// execute the request to the backend
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return false, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return false, nil, nil
	}

	return true, resp, nil
}

// shouldValidate checks if the content has changed since we cached it
func shouldValidate(c *cache.ContentObject, d time.Duration) bool {
	if c.CachedTimestamp() == 0 {
		return true
	}

	return time.Now().Sub(time.Unix(c.CachedTimestamp(), 0).Add(d)) > 0
}

// respond sends the response back to the client
func respond(w http.ResponseWriter, hh http.Header, body []byte) error {
	for k, v := range hh {
		w.Header().Set(k, v[0])
	}

	w.WriteHeader(http.StatusOK)
	w.Write(body)
	return nil
}

// httpHandler is the main handler for the CDN
func (c *CDN) httpHandler(w http.ResponseWriter, req *http.Request) {
	start := time.Now()

	host := req.Host
	defer requestDuration.WithLabelValues(host).Observe(time.Since(start).Seconds())

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
		cacheMetric.WithLabelValues(host, "lookup_error").Inc()
	}

	var reqBody []byte
	if req.Body != nil {
		reqBody, err = ioutil.ReadAll(req.Body)
		if err != nil {
			logrus.Errorf("error reading request body: %s", err)
			requestsMetric.WithLabelValues(host, strconv.Itoa(http.StatusBadRequest), "error").Inc()
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if found {
		// check if the URL needs to be validated. Validate if 15m have elapsed
		if shouldValidate(content, 15*time.Second) {
			// validate
			tmpReq, err := http.NewRequest(req.Method, fr, bytes.NewReader(reqBody))
			if err != nil {
				logrus.Errorf("error creating validation request: %s", err)
				validationErrorsMetric.WithLabelValues(host).Inc()
				w.WriteHeader(http.StatusInternalServerError)
			}
			for k, v := range req.Header {
				req.Header.Set(k, strings.Join(v, " "))
			}

			validated, resp, err := c.validate(tmpReq)
			if err != nil {
				logrus.Errorf("error validating cached item: %s", err)
				validationErrorsMetric.WithLabelValues(host).Inc()
				w.WriteHeader(http.StatusInternalServerError)
			}
			if validated && resp != nil {
				validationMetric.WithLabelValues(host).Inc()
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					logrus.Errorf("error reading validated body: %s", err)
					validationErrorsMetric.WithLabelValues(host).Inc()
					w.WriteHeader(http.StatusInternalServerError)
				}
				respond(w, resp.Header, body)
			}
			return
		}

		logrus.Debugf("cache hit: %s (%s)", fr, content.ContentType)
		cacheMetric.WithLabelValues(host, "hit").Inc()
		requestsMetric.WithLabelValues(host, strconv.Itoa(http.StatusOK), "success").Inc()

		hh := http.Header{}
		for k, v := range content.Headers() {
			hh[k] = []string{v}
		}

		respond(w, hh, content.Content())
		return
	}

	// cache miss, fetch content again
	logrus.Debugf("cache miss: %s", fr)
	r, err := http.NewRequest(req.Method, fr, bytes.NewReader(reqBody))
	if err != nil {
		logrus.Errorf("error creating a new proxy request: %s", err)
		requestsMetric.WithLabelValues(host, strconv.Itoa(http.StatusBadRequest), "error").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// propagating all headers to the backend request
	for k, v := range req.Header {
		logrus.Debugf("propagating headers to backend %s: %s", k, v[0])
		r.Header.Add(k, v[0])
	}

	// execute the request to the backend
	resp, err := c.httpClient.Do(r)
	if err != nil {
		logrus.Errorf("error proxying request: %s", err)
		requestsMetric.WithLabelValues(host, strconv.Itoa(http.StatusBadRequest), "error").Inc()
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	// read the response body
	rb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("error reading response body: %s", err)
		requestsMetric.WithLabelValues(host, strconv.Itoa(http.StatusInternalServerError), "error").Inc()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	requestsMetric.WithLabelValues(host, strconv.Itoa(resp.StatusCode), "success").Inc()

	// respond to client as soon as possible
	respond(w, resp.Header, rb)

	cachable, cii := c.isCachable(resp.Header)
	if !cachable {
		return
	}

	logrus.Debugf("[%s] Content-type: %s", fr, cii.ContentType)
	ccParserMetric.WithLabelValues(host, "content_type_present").Inc()
	ccParserMetric.WithLabelValues(host, "content_type_cachable").Inc()
	ccParserMetric.WithLabelValues(host, "cache_control_cachable").Inc()
	logrus.Infof("storing a new object in cache: %s (%s)", fr, cii.ContentType)

	// we also want to store the headers
	hh := cleanHeadersMap(respHeadersToMap(resp))
	co := cache.NewContentObject(rb, cii.ContentType, hh, cii.MaxAge, time.Now().Unix())
	// avoid keeping the handler busy while storing the object in cache
	// Prefer freeing up the handler as fast as possible rather than checking if
	// there was an error storing the object. It will be picked up via metrics/logs.
	go func() {
		err := c.cache.Store(reqURL, co)
		if err != nil {
			logrus.Errorf("error storing cache item %s: %s", reqURL, err)
			cacheMetric.WithLabelValues(host, "store_error").Inc()
		}
		logrus.Debugf("successfully stored item %s", reqURL)
		cacheMetric.WithLabelValues(host, "stored").Inc()
	}()
}
