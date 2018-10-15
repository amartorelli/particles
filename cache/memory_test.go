package cache

import (
	"bytes"
	"testing"
)

func TestIsCachableContentType(t *testing.T) {
	tt := []struct {
		ct       string
		cachable bool
		errMsg   string
	}{
		{"text/css", true, "text/css should be cached"},
		{"image/gif", true, "image/gif should be cached"},
		{"video/mpeg", true, "image/gif should be cached"},
		{"application/javascript", true, "application/javascript should be cached"},
		{"text/html", false, "text/html should not be cached"},
		{"font/otf", false, "font/otf should not be cached"},
		{"application/xml", false, "application/xml should not be cached"},
	}
	cc := DefaultConf()
	c, err := NewCache(cc)
	if err != nil {
		t.Error(err)
	}

	for _, tc := range tt {
		cachable := c.IsCachableContentType(tc.ct)
		if cachable != tc.cachable {
			t.Errorf("%s, result: %t, expected: %ts", tc.errMsg, cachable, tc.cachable)
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
		{"present but expired item", "www.expired-item.com", false, []byte("expired"), errExpiredItem},
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
	err = c.Store("www.valid.com", validCO)
	if err != nil {
		t.Error(err)
	}

	notFoundCO := NewContentObject(
		[]byte("notfound"),
		"application/javascript",
		map[string]string{"Content-Type": "application/javascript"},
		10,
	)
	err = c.Store("www.not-found.com", notFoundCO)
	if err != nil {
		t.Error(err)
	}

	invalidContentCO := NewContentObject(
		[]byte("invalid"),
		"application/javascript",
		map[string]string{"Content-Type": "application/javascript"},
		10,
	)
	err = c.Store("www.invalid-content.com", invalidContentCO)
	if err != nil {
		t.Error(err)
	}

	expiredCO := NewContentObject(
		[]byte("expired"),
		"application/word",
		map[string]string{"Content-Type": "application/javascript"},
		-3600,
	)
	err = c.Store("www.invalid-header.com", expiredCO)
	if err != nil {
		t.Error(err)
	}

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
		err      error
	}{
		{"default item", "www.default.com", []byte("default item"), 0, nil},
		{"requires freeup space", "www.freeup.com", []byte("free up space"), 0, nil},
		{"custom ttl", "www.customttl.com", []byte("custom ttl"), 45, nil},
		// {"can't free up memory", "www.cantfree.com", []byte("cant free up memory"), 0, errFreeMemory},
		{"item can't fit in memory", "www.cantfit.com", []byte("this item can't fit in memory"), 0, errNotEnoughMemory},
	}

	cc := DefaultConf()
	// cc.Options["force_purge"] = "false"
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
		if err != tc.err {
			t.Fatalf("unexpected error value from store: %v, expected %v", err, tc.err)
		}

		r, found, err := c.Lookup(tc.key)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			continue
		}

		compTTL := 86400
		if tc.ttl != 0 {
			compTTL = tc.ttl
		}
		if r.TTL() != compTTL {
			t.Errorf("ttl should be %d, but has %d", compTTL, r.TTL())
		}
	}
}

func TestPurge(t *testing.T) {
	tt := []struct {
		key string
		err error
	}{
		{"www.existing.com", nil},
		{"www.non-existing.com", errNotFound},
	}

	cc := DefaultConf()
	c, err := NewCache(cc)
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
	err = c.Store("www.existing.com", co)
	if err != nil {
		t.Error(err)
	}

	for _, tc := range tt {
		err = c.Purge(tc.key)
		if err != tc.err {
			t.Errorf("%s: %s", tc.key, err)
		}
	}
}
