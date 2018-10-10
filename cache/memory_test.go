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
