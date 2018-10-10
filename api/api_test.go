package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"particles/cache"
	"testing"
)

func TestPurgeHandler(t *testing.T) {
	ac := DefaultConf()
	cc := cache.DefaultConf()
	c, err := cache.NewCache(cc)
	if err != nil {
		t.Error(err)
	}

	a, err := NewAPI(ac, c)
	if err != nil {
		t.Error(err)
	}

	tt := []struct {
		method string
		data   []byte
		code   int
		err    error
	}{
		{"GET", []byte(""), http.StatusMethodNotAllowed, fmt.Errorf("A get request should be not allowed")},
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
			t.Error(tc.err)
		}
	}
}
