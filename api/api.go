package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"particles/cache"
	"particles/util"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// API represents the CDN API ojbect
type API struct {
	server   *http.Server
	mux      *http.ServeMux
	certFile string
	keyFile  string
	cache    cache.Cache
}

// NewAPI returns a new API object
func NewAPI(conf Conf, cache cache.Cache) (*API, error) {
	mux := http.NewServeMux()
	lHTTPAddr := fmt.Sprintf("%s:%d", conf.Address, conf.Port)
	s := &http.Server{
		Addr:           lHTTPAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return &API{server: s, mux: mux, certFile: conf.CertFile, keyFile: conf.KeyFile, cache: cache}, nil
}

// Start starts the API server
func (a *API) Start() error {
	a.mux.Handle("/metrics", promhttp.Handler())
	a.mux.Handle("/purge", util.WithLogging(a.purgeHandler))
	// if certificates have been configured, start on HTTPS
	// otherwise fold back to normal HTTP
	if a.certFile != "" && a.keyFile != "" {
		logrus.Infof("Starting API on %s (HTTPS)", a.server.Addr)
		err := a.server.ListenAndServeTLS(a.certFile, a.keyFile)
		if err != nil {
			return err
		}
	} else {
		logrus.Infof("Starting API on %s (HTTP)", a.server.Addr)
		err := a.server.ListenAndServe()
		if err != nil {
			return err
		}
	}
	return nil
}

// Shutdown terminates the API server in a clean way
func (a *API) Shutdown(ctx context.Context) error {
	err := a.server.Shutdown(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Response is used to json encode the API response
type Response struct {
	Message string
}

// PurgeRequest is used to receive a call to purge an item from the cache
type PurgeRequest struct {
	Resource string `json:"resource"`
}

// purgeHandler exposes an endpoint to purge items from the cache
func (a *API) purgeHandler(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	defer purgeDuration.Observe(time.Since(start).Seconds())

	defer req.Body.Close()
	r := Response{}
	w.Header().Set("Content-Type", "application/json")
	if req.Method != http.MethodPost {
		logrus.Error("method not allowed")
		purgeMetric.WithLabelValues(strconv.Itoa(http.StatusMethodNotAllowed)).Inc()
		r.Message = "method not allowed"
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(r)
		return
	}

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Error("unable to read request body")
		purgeMetric.WithLabelValues(strconv.Itoa(http.StatusInternalServerError)).Inc()
		r.Message = "internal error"
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(r)
		return
	}

	pr := PurgeRequest{}
	err = json.Unmarshal(b, pr)
	if err != nil {
		logrus.Error("unable to parse purge request")
		purgeMetric.WithLabelValues(strconv.Itoa(http.StatusBadRequest)).Inc()
		r.Message = "unable to parse purge request"
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(r)
		return
	}

	err = a.cache.Purge(pr.Resource)
	if err != nil {
		logrus.Error("unable purge item from cache")
		purgeMetric.WithLabelValues(strconv.Itoa(http.StatusInternalServerError)).Inc()
		r.Message = "internal error"
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(r)
		return
	}

	logrus.Infof("successfully purged item %s", pr.Resource)
	r.Message = "successfully purged item from cache"
	purgeMetric.WithLabelValues(strconv.Itoa(http.StatusOK)).Inc()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(r)
	return
}
