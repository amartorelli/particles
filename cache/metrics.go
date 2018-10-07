package cache

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	lookupMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cdn_cache_lookup_total",
		Help: "Lookup count",
	}, []string{"status"})

	storeMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cdn_cache_store_total",
		Help: "Store count",
	}, []string{"status"})

	purgeMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cdn_cache_purge_total",
		Help: "Purge count",
	}, []string{"status"})
)
