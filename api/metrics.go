package api

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	purgeMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cdn_api_purge_total",
		Help: "Purge requests received by the API",
	}, []string{"code"})

	purgeDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cdn_api_purge_seconds",
		Help: "Purge requests duration",
	})
)
