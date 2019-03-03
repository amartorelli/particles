package util

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HandlerWithLogging enables logging for requests
func HandlerWithLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start).Seconds()
		logrus.Infof("%s %s%s %s %fs", r.RemoteAddr, r.Host, r.URL, r.UserAgent(), duration)
	}
}
