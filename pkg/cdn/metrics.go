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

	cacheMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_requests_cache_total",
		Help: "Requests received by the CDN",
	}, []string{"status"})

	ccParserMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_requests_contenttype_parser_total",
		Help: "Status of the parser when analysing the response headers",
	}, []string{"event"})

	validationMetric = promauto.NewCounter(prometheus.CounterOpts{
		Name: "particles_validations_total",
		Help: "Number of cache validations needed",
	})

	validationErrorsMetric = promauto.NewCounter(prometheus.CounterOpts{
		Name: "particles_validation_errors_total",
		Help: "Number of cache validations needed",
	})
)
