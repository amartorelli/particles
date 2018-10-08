package cache

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/sirupsen/logrus"
)

// MemcachedCache represents a cache object
type MemcachedCache struct {
	endpoints         string
	contentTypeRegexp *regexp.Regexp
	mc                *memcache.Client
	defaultTTL        int
}

// NewMemcachedCache initialises a new cache
func NewMemcachedCache(options map[string]string) (*MemcachedCache, error) {
	// endpoints
	endpoints := "127.0.0.1:11211"
	v, ok := options["endpoints"]
	if ok {
		endpoints = v
	}

	// patterns
	var patterns string
	v, ok = options["patterns"]
	if ok {
		patterns = v
	}
	regex, err := contentTypeRegex(patterns)
	if err != nil {
		return nil, err
	}

	// memcached connection
	mc := memcache.New(endpoints)

	return &MemcachedCache{endpoints: endpoints, contentTypeRegexp: regex, mc: mc}, nil
}

// IsCachableContentType returns true in case the content type is one that can be cached
func (c *MemcachedCache) IsCachableContentType(contentType string) bool {
	return c.contentTypeRegexp.MatchString(contentType)
}

// Lookup returns the content if present and a boolean to represent if it's been found
// In memcache we store the content type in the value in the form:
// value ="content-type|bytes" so we split on "|"
func (c *MemcachedCache) Lookup(key string) (*ContentObject, bool, error) {
	i, err := c.mc.Get(key)
	if err == memcache.ErrCacheMiss {
		lookupMetric.WithLabelValues("miss").Inc()
		return nil, false, nil
	}
	if err != nil {
		lookupMetric.WithLabelValues("error").Inc()
		return nil, false, err
	}
	sep := bytes.Index(i.Value, []byte("|"))
	if sep == -1 {
		lookupMetric.WithLabelValues("error").Inc()
		return nil, false, fmt.Errorf("invalid object fetched from memcached")
	}

	contentType := []byte(i.Value[0:sep])
	value := []byte(i.Value[sep+1:])
	// logrus.Errorf("CONTENTTYPE:", contentType)
	// logrus.Errorf("VALUE:", value)
	lookupMetric.WithLabelValues("success").Inc()
	// TODO: fix me
	return NewContentObject(value, string(contentType), make(map[string]string, 0), int(i.Expiration)), true, nil
}

// Store inserts a new entry into the cache
func (c *MemcachedCache) Store(key string, co *ContentObject) error {
	v := append([]byte(co.ContentType), []byte("|")...)
	logrus.Error("STORED:", append(v, co.Content()...))
	i := memcache.Item{
		Key:        key,
		Value:      append(v, co.Content()...),
		Expiration: int32(co.TTL()),
	}

	err := c.mc.Add(&i)
	if err != memcache.ErrNotStored {
		storeMetric.WithLabelValues("error").Inc()
		return err
	}

	storeMetric.WithLabelValues("success").Inc()
	return nil
}

// Purge deletes an item from the cache
func (c *MemcachedCache) Purge(key string) error {
	err := c.mc.Delete(key)
	if err == memcache.ErrCacheMiss {
		purgeMetric.WithLabelValues("miss").Inc()
		return nil
	}

	if err != nil {
		purgeMetric.WithLabelValues("error").Inc()
		return err
	}

	purgeMetric.WithLabelValues("success").Inc()
	return nil
}
