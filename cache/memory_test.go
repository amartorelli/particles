package cache

import (
	"bytes"
	"fmt"
	"testing"
)

func TestIsCachableContentType(t *testing.T) {
	tt := []struct {
		ct       string
		cachable bool
		err      error
	}{
		{"text/css", true, fmt.Errorf("text/css should be cached")},
		{"image/gif", true, fmt.Errorf("image/gif should be cached")},
		{"video/mpeg", true, fmt.Errorf("image/gif should be cached")},
		{"application/javascript", true, fmt.Errorf("application/javascript should be cached")},
		{"text/html", false, fmt.Errorf("text/html should not be cached")},
		{"font/otf", false, fmt.Errorf("font/otf should not be cached")},
		{"application/xml", false, fmt.Errorf("application/xml should not be cached")},
	}
	cc := DefaultConf()
	c, err := NewCache(cc)
	if err != nil {
		t.Error(err)
	}

	for _, tc := range tt {
		cachable := c.IsCachableContentType(tc.ct)
		if cachable != tc.cachable {
			t.Errorf("%s, result: %t, expected: %t", tc.err, cachable, tc.cachable)
		}
	}
}

func TestLookup(t *testing.T) {
	tt := []struct {
		testCase string
		key      string
		present  bool
		data     []byte
		err      error
	}{
		{"present and valid item", "www.valid.com", true, []byte("valid"), nil},
		{"not found", "www.notfound.com", false, []byte("notfound"), nil},
		{"present item, wrong data", "www.invalid-content.com", true, []byte("invalid"), nil},
		{"present but expired item", "www.expired-item.com", false, []byte("expired"), fmt.Errorf("expired entry")},
	}

	cc := DefaultConf()
	c, err := NewCache(cc)
	if err != nil {
		t.Error(err)
	}

	// store sample items in cache
	validCO := NewContentObject(
		[]byte("valid"),
		"application/javascript",
		map[string]string{"Content-Type": "application/javascript"},
		10,
	)
	c.Store("www.valid.com", validCO)

	notFoundCO := NewContentObject(
		[]byte("notfound"),
		"application/javascript",
		map[string]string{"Content-Type": "application/javascript"},
		10,
	)
	c.Store("www.not-found.com", notFoundCO)

	invalidContentCO := NewContentObject(
		[]byte("invalid"),
		"application/javascript",
		map[string]string{"Content-Type": "application/javascript"},
		10,
	)
	c.Store("www.invalid-content.com", invalidContentCO)

	expiredCO := NewContentObject(
		[]byte("expired"),
		"application/word",
		map[string]string{"Content-Type": "application/javascript"},
		-3600,
	)
	c.Store("www.invalid-header.com", expiredCO)

	for _, tc := range tt {
		co, found, err := c.Lookup(tc.key)

		// check item is present
		if found != tc.present {
			t.Errorf("%s: found %t, expected %t", tc.err, found, tc.present)
		}

		if !found {
			continue
		}

		// check content is what's expected
		if bytes.Compare(co.content, tc.data) != 0 {
			t.Errorf("content doesn't match: expected %s, found %s", tc.data, co.content)
		}

		// check returned error
		if err != tc.err {
			t.Errorf("expecting error to be %s, received %s", tc.err, err)
		}
	}
}

func TestStore(t *testing.T) {
	tt := []struct {
		testCase string
		key      string
		data     []byte
		ttl      int
	}{
		{"default item", "www.default.com", []byte("default item"), 0},
		{"requires freeup space", "www.freeup.com", []byte("free up space"), 0},
		{"custom ttl", "www.customttl.com", []byte("custom ttl"), 45},
	}

	cc := DefaultConf()
	cc.Options["memory_limit"] = "20"
	c, err := NewCache(cc)
	if err != nil {
		t.Error(err)
	}

	for _, tc := range tt {
		co := NewContentObject(
			tc.data,
			"application/javascript",
			map[string]string{"Content-Type": "application/javascript"},
			tc.ttl,
		)
		err := c.Store(tc.key, co)
		if err != nil {
			t.Errorf("unexpected error value from store: %v", err)
		}

		r, found, err := c.Lookup(tc.key)
		if err != nil {
			t.Error(err)
		}
		if !found {
			t.Errorf("item not found")
		}
		compTTL := 86400
		if tc.ttl != 0 {
			compTTL = tc.ttl
		}
		if r.TTL() != compTTL {
			t.Errorf("ttl should be %d, but has %d", compTTL, r.TTL())
		}
	}

	tterr := []struct {
		testCase string
		key      string
		data     []byte
		ttl      int
		err      error
	}{
		{"can't free up memory", "www.cantfree.com", []byte("cant free up memory"), 0, fmt.Errorf("unable to free enough memory (0/19)")},
		{"item can't fit in memory", "www.cantfit.com", []byte("this item can't fit in memory"), 0, fmt.Errorf("item www.cantfit.com can't fit in memory")},
	}

	cc = DefaultConf()
	cc.Options["force_purge"] = "false"
	cc.Options["memory_limit"] = "20"
	c, err = NewCache(cc)
	if err != nil {
		t.Error(err)
	}

	// load sample item
	co := NewContentObject(
		[]byte("01234567890123"),
		"application/javascript",
		map[string]string{"Content-Type": "application/javascript"},
		0,
	)
	err = c.Store("filler", co)

	for _, tc := range tterr {
		co := NewContentObject(
			tc.data,
			"application/javascript",
			map[string]string{"Content-Type": "application/javascript"},
			tc.ttl,
		)
		err := c.Store(tc.key, co)
		if err.Error() != tc.err.Error() {
			t.Errorf("unexpected error value from store: %v, expected %v", err, tc.err)
		}
	}
}
