package cache

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	lookupMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_cache_lookup_total",
		Help: "Cache lookup count",
	}, []string{"domain", "type", "status"})

	lookupDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "particles_cache_lookup_seconds",
		Help: "Cache lookup duration",
	}, []string{"domain", "type"})

	storeMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_cache_store_total",
		Help: "Store count",
	}, []string{"domain", "type", "status"})

	storeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "particles_cache_store_seconds",
		Help: "Cache store duration",
	}, []string{"domain", "type"})

	purgeMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_cache_purge_total",
		Help: "Purge count",
	}, []string{"domain", "type", "status"})

	purgeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "particles_cache_purge_seconds",
		Help: "Cache purge duration",
	}, []string{"domain", "type"})
)
