package cdn

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestsMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_requests_total",
		Help: "Requests received by the CDN",
	}, []string{"domain", "code", "status"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "particles_requests_seconds",
		Help: "Requests duration received by the CDN",
	}, []string{"domain"})

	cacheMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_requests_cache_total",
		Help: "Requests received by the CDN",
	}, []string{"domain", "status"})

	ccParserMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_requests_contenttype_parser_total",
		Help: "Status of the parser when analysing the response headers",
	}, []string{"domain", "event"})

	validationMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_validations_total",
		Help: "Number of cache validations needed",
	}, []string{"domain"})

	validationErrorsMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "particles_validation_errors_total",
		Help: "Number of cache validations needed",
	}, []string{"domain"})
)
