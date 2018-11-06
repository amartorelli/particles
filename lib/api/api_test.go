package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amartorelli/particles/lib/cache"
)

func TestPurgeHandler(t *testing.T) {
	notFoundPR, err := json.Marshal(PurgeRequest{Resource: "www.wrong.com"})
	if err != nil {
		t.Error(err)
	}

	foundPR, err := json.Marshal(PurgeRequest{Resource: "www.example.com"})
	if err != nil {
		t.Error(err)
	}

	tt := []struct {
		method string
		data   []byte
		code   int
		errMsg string
	}{
		{"GET", []byte(""), http.StatusMethodNotAllowed, "A get request should be not allowed"},
		{"POST", []byte("bad request"), http.StatusBadRequest, "An invalid purge request should return a bad request"},
		{"POST", notFoundPR, http.StatusInternalServerError, "A request trying to purge an item that isn't present should return an internal error"},
		{"POST", foundPR, http.StatusOK, "A request trying to purge an item that is present should return an OK code"},
	}

	ac := DefaultConf()
	cc := cache.DefaultConf()
	c, err := cache.NewCache(cc)
	if err != nil {
		t.Error(err)
	}

	// store a sample item in cache
	co := cache.NewContentObject(
		[]byte("test"),
		"text/html",
		make(map[string]string),
		10,
	)
	c.Store("www.example.com", co)

	a, err := NewAPI(ac, c)
	if err != nil {
		t.Error(err)
	}

	for _, tc := range tt {
		body := bytes.NewReader(tc.data)
		req, err := http.NewRequest(tc.method, "/purge", body)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		a.purgeHandler(rr, req)

		if rr.Code != tc.code {
			t.Errorf("%s: expected %d, received %d", tc.errMsg, tc.code, rr.Code)
		}
	}
}
