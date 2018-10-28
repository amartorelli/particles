package cdn

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"particles/lib/cache"
	"strconv"
	"strings"
	"testing"
	"time"
)

var exampleContent = `<!doctype html>
<html>
<head>
    <title>Example Domain</title>

    <meta charset="utf-8" />
    <meta http-equiv="Content-type" content="text/html; charset=utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style type="text/css">
    body {
        background-color: #f0f0f2;
        margin: 0;
        padding: 0;
        font-family: "Open Sans", "Helvetica Neue", Helvetica, Arial, sans-serif;
        
    }
    div {
        width: 600px;
        margin: 5em auto;
        padding: 50px;
        background-color: #fff;
        border-radius: 1em;
    }
    a:link, a:visited {
        color: #38488f;
        text-decoration: none;
    }
    @media (max-width: 700px) {
        body {
            background-color: #fff;
        }
        div {
            width: auto;
            margin: 0 auto;
            border-radius: 0;
            padding: 1em;
        }
    }
    </style>    
</head>

<body>
<div>
    <h1>Example Domain</h1>
    <p>This domain is established to be used for illustrative examples in documents. You may use this
    domain in examples without prior coordination or asking for permission.</p>
    <p><a href="http://www.iana.org/domains/example">More information...</a></p>
</div>
</body>
</html>
`

func TestHTTPHandler(t *testing.T) {
	// Starting fake website
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		fmt.Fprintf(w, exampleContent)
	})
	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/css")
		fmt.Fprintf(w, "not cached")
	})
	mux.HandleFunc("/cachable.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/css")
		w.Header().Add("Cache-Control", "public")
		fmt.Fprintf(w, "cachable content")
	})
	mux.HandleFunc("/private.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/css")
		w.Header().Add("Cache-Control", "private")
		fmt.Fprintf(w, "private content")
	})
	mux.HandleFunc("/maxage.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/css")
		w.Header().Add("Cache-Control", "public, max-age=600")
		fmt.Fprintf(w, "max-age")
	})

	s := &http.Server{
		Addr:           ":7887",
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	defer s.Close()

	go s.ListenAndServe()

	c := DefaultConf()
	bc := BackendConf{
		Name:   "example",
		Domain: "www.example.com",
		IP:     "127.0.0.1",
		Port:   7887,
	}
	c.HTTP.Backends = []BackendConf{bc}
	cdn, err := NewCDN(c)
	if err != nil {
		t.Error(err)
	}

	// test the request of an object that's stored in cache
	co := cache.NewContentObject(
		[]byte("cached"),
		"text/css",
		make(map[string]string),
		10,
	)
	cdn.cache.Store("http://www.example.com/style.css", co)

	// Override client connection to use local temporary server
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 10 * time.Second,
		DualStack: true,
	}
	http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		// the address always includes the port, so we split
		addrParts := strings.Split(addr, ":")
		host := addrParts[0]
		port := addrParts[1]

		// check we manage that endpoint
		e, ok := cdn.endpoints[host]
		if !ok {
			return nil, fmt.Errorf("unhandled endpoint")
		}

		// override IP and/or port if defined
		if e.IP != "" {
			host = e.IP
		}
		if e.Port > 0 {
			port = strconv.Itoa(e.Port)
		}

		addr = fmt.Sprintf("%s:%s", host, port)
		return dialer.DialContext(ctx, network, addr)
	}

	req, err := http.NewRequest("GET", "http://www.example.com/style.css", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	cdn.httpHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("cached object, expected status code %d, received %d", http.StatusOK, rr.Code)
	}

	if rr.Body.String() != "cached" {
		t.Errorf("cached object, expected body 'cached', received '%s'", rr.Body.String())
	}

	// non cachable content
	rr = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "http://www.example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}

	cdn.httpHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("non cachable object, expected status code %d, received %d", http.StatusOK, rr.Code)
	}

	if rr.Body.String() != exampleContent {
		t.Errorf("non cachable object, expected body is the exampleContent, received '%s'", rr.Body.String())
	}

	_, found, err := cdn.cache.Lookup("http://www.example.com/")
	if err != nil {
		t.Error(err)
	}
	if found {
		t.Errorf("http://www.example.com/ is not cachable and shouldn't be found in cache, but it's been found")
	}

	// cachable content
	rr = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "http://www.example.com/cachable.css", nil)
	if err != nil {
		t.Fatal(err)
	}

	cdn.httpHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("cachable object, expected status code %d, received %d", http.StatusOK, rr.Code)
	}

	if rr.Body.String() != "cachable content" {
		t.Errorf("non cachable object, expected body is 'cachable content', received '%s'", rr.Body.String())
	}

	time.Sleep(1 * time.Second)
	_, found, err = cdn.cache.Lookup("http://www.example.com/cachable.css")
	if err != nil {
		t.Error(err)
	}
	if !found {
		t.Errorf("http://www.example.com/cachable.css is cachable and should have been added to the cache, but it hasn't been found")
	}

	// private content
	rr = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "http://www.example.com/private.css", nil)
	if err != nil {
		t.Fatal(err)
	}

	cdn.httpHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("private object, expected status code %d, received %d", http.StatusOK, rr.Code)
	}

	if rr.Body.String() != "private content" {
		t.Errorf("private object, expected body is 'private content', received '%s'", rr.Body.String())
	}

	time.Sleep(1 * time.Second)
	_, found, err = cdn.cache.Lookup("http://www.example.com/private.css")
	if err != nil {
		t.Error(err)
	}
	if found {
		t.Errorf("http://www.example.com/private.css isn't cachable and should not be added to the cache, but it's been found")
	}

	// max-age
	rr = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "http://www.example.com/maxage.css", nil)
	if err != nil {
		t.Fatal(err)
	}

	cdn.httpHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("public object with max-age, expected status code %d, received %d", http.StatusOK, rr.Code)
	}

	if rr.Body.String() != "max-age" {
		t.Errorf("public object with max-age, expected body is 'max-age', received '%s'", rr.Body.String())
	}

	time.Sleep(1 * time.Second)
	item, found, err := cdn.cache.Lookup("http://www.example.com/maxage.css")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("http://www.example.com/maxage.css is cachable and should be added to the cache, but it's not been found")
	}

	if item.TTL() != 600 {
		t.Errorf("http://www.example.com/maxage.css expected TTL is 600, found %d", item.TTL())
	}

	// TODO: test headers are correctly propagated to the cache and returned when reading from cache
}

func TestIsCachable(t *testing.T) {
	tt := []struct {
		header   string
		cachable bool
		errMsg   string
	}{
		{"public", true, "public Cache-Control header should be cachable"},
		{"public, max-age=3600", true, "public Cache-Control header with max-age should be cachable"},
		{"private", false, "private Cache-Control header should not be cachable"},
		{"no-store", false, "no-store Cache-Control header should not be cachable"},
		{"no-cache", false, "no-cache Cache-Control header should not be cachable"},
	}

	for _, tc := range tt {
		if isCachable(tc.header) != tc.cachable {
			t.Error(tc.errMsg)
		}
	}
}

func TestGetMaxAge(t *testing.T) {
	tt := []struct {
		header string
		expTTL int
		errMsg string
	}{
		{"public", 0, "expected max-age 0 because unspecified"},
		{"public, max-age=3600", 3600, "public Cache-Control header with max-age=3600 should return 3600"},
		{"private, max-age=3600", 3600, "private Cache-Control header with max-age=3600 should return 3600"},
	}

	for _, tc := range tt {
		if getMaxAge(tc.header) != tc.expTTL {
			t.Error(tc.errMsg)
		}
	}
}
