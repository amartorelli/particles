package cache

import (
	"bytes"
	"encoding/gob"
	"errors"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/bradfitz/gomemcache/memcache"
)

var (
	errDecodingItem = errors.New("error decoding item")
	errStoringItem  = errors.New("error storing item")
)

// MemcachedCache represents a cache object
type MemcachedCache struct {
	endpoints         string
	contentTypeRegexp *regexp.Regexp
	mc                *memcache.Client
	defaultTTL        int
}

// MemcachedItem is the structure used to serialize data into memcache
type MemcachedItem struct {
	Content         []byte
	Headers         map[string]string
	ContentType     string
	CachedTimestamp int64
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
	start := time.Now()
	defer lookupDuration.WithLabelValues("memcached").Observe(time.Since(start).Seconds())

	i, err := c.mc.Get(key)
	if err == memcache.ErrCacheMiss {
		logrus.Debugf("cache miss for %s: %s", key, err)
		lookupMetric.WithLabelValues("memcached", "miss").Inc()
		return nil, false, nil
	}
	if err != nil {
		logrus.Debugf("error during the lookup of %s: %s", key, err)
		lookupMetric.WithLabelValues("memcached", "error").Inc()
		return nil, false, err
	}

	var mi MemcachedItem
	buf := bytes.NewBuffer(i.Value)

	dec := gob.NewDecoder(buf)
	err = dec.Decode(&mi)
	if err != nil {
		logrus.Debugf("error decoding item for %s: %s", key, err)
		lookupMetric.WithLabelValues("memcached", "error").Inc()
		return nil, false, errDecodingItem
	}
	lookupMetric.WithLabelValues("memcached", "success").Inc()

	return NewContentObject([]byte(mi.Content), string(mi.ContentType), mi.Headers, int(i.Expiration), int64(mi.CachedTimestamp)), true, nil
}

// Store inserts a new entry into the cache
func (c *MemcachedCache) Store(key string, co *ContentObject) error {
	start := time.Now()
	defer storeDuration.WithLabelValues("memcached").Observe(time.Since(start).Seconds())

	var buf bytes.Buffer
	mi := &MemcachedItem{Content: co.Content(), Headers: co.Headers(), ContentType: co.ContentType, CachedTimestamp: time.Now().Unix()}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(mi)
	if err != nil {
		logrus.Debugf("error encoding item to store for %s: %s", key, err)
		return errStoringItem
	}

	i := memcache.Item{
		Key:        key,
		Value:      buf.Bytes(),
		Expiration: int32(co.TTL()),
	}

	err = c.mc.Set(&i)
	if err != nil {
		logrus.Debugf("error storing item %s: %s", key, err)
		storeMetric.WithLabelValues("memcached", "error").Inc()
		return err
	}

	storeMetric.WithLabelValues("memcached", "success").Inc()
	return nil
}

// Purge deletes an item from the cache
func (c *MemcachedCache) Purge(key string) error {
	start := time.Now()
	defer purgeDuration.WithLabelValues("memcached").Observe(time.Since(start).Seconds())

	err := c.mc.Delete(key)
	if err == memcache.ErrCacheMiss {
		purgeMetric.WithLabelValues("memcached", "miss").Inc()
		return nil
	}

	if err != nil {
		purgeMetric.WithLabelValues("memcached", "error").Inc()
		return err
	}

	purgeMetric.WithLabelValues("memcached", "success").Inc()
	return nil
}
