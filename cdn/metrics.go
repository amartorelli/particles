package cdn

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestsMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_requests_total",
		Help: "Requests received by the CDN",
	}, []string{"code", "status"})

	requestDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "particles_requests_seconds",
		Help: "Requests duration received by the CDN",
	})
)
