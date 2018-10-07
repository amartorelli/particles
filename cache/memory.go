package cache

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// defaultMemLimit is the maximum amount of memory used by the cache
	defaultMemLimit = 1073741824 // 1GB default memory limit
)

// MemoryCache represents a cache object
type MemoryCache struct {
	objs              map[string]*ContentObject
	contentTypeRegexp *regexp.Regexp
	defaultTTL        int
	objsMutex         sync.RWMutex
	memLimit          int
	memSize           int
	hits              int
	misses            int
}

// MemoryCacheConfig is how the configuration for the memory cache is represented in the config file
type MemoryCacheConfig struct {
	MemoryLimit int      `yaml:"memory_limit"` // mandatory, how much memory in bytes to use
	TTL         int      `yaml:"ttl"`          // optional, how long each entry is cached for
	Patterns    []string `yaml:"patterns"`     // optional, content-type patterns
}

// NewMemoryCache initialises a new cache
func NewMemoryCache(options map[string]string) (*MemoryCache, error) {
	// memory limit
	ml := defaultMemLimit
	v, ok := options["memory_limit"]
	if ok {
		tmp, err := strconv.Atoi(options["memory_limit"])
		if err != nil {
			return nil, fmt.Errorf("error parsing memory_limit: %s", err)
		}
		ml = tmp
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

	return &MemoryCache{objs: make(map[string]*ContentObject), contentTypeRegexp: regex, objsMutex: sync.RWMutex{}, memLimit: ml}, nil
}

// IsCachableContentType returns true in case the content type is one that can be cached
func (c *MemoryCache) IsCachableContentType(contentType string) bool {
	return c.contentTypeRegexp.MatchString(contentType)
}

// Lookup returns the content if present and a boolean to represent if it's been found
func (c *MemoryCache) Lookup(key string) (*ContentObject, bool, error) {
	c.objsMutex.RLock()
	co, found := c.objs[key]
	if found {
		// if the entry has expired, don't return it and delete it
		if time.Now().After(co.Expiration()) {
			lookupMetric.WithLabelValues("miss").Inc()
			c.misses++
			delete(c.objs, key)
			c.objsMutex.RUnlock()
			return nil, false, fmt.Errorf("expired entry")
		}
		lookupMetric.WithLabelValues("hit").Inc()
		co.hits++
		c.hits++
	} else {
		lookupMetric.WithLabelValues("miss").Inc()
		c.misses++
	}
	c.objsMutex.RUnlock()
	return co, found, nil
}

// freeMemory frees up some space in the hash map to at least fit a new object of size bytes
// By default it deletes entries that have been hit less than 10% of the current total hits received
// by the cache
func (c *MemoryCache) freeMemory(size int) error {
	logrus.Debugf("freeing up memory to allocate %d bytes", size)
	var tbd []string
	var fs int
	// free memory by removing an entry that has been hit less than
	// 10% of total hits
	tenPercentHits := 10 * c.hits / 100

	for k, co := range c.objs {
		if co.hits < tenPercentHits {
			tbd = append(tbd, k)
			fs = fs + co.Size()
		}
		if fs > size {
			break
		}
	}
	c.purgeEntries(tbd)
	if fs < size {
		return fmt.Errorf("unable to free enough memory (%d/%d)", fs, size)
	}
	return nil
}

// PurgeEntries deletes a batch of keys
func (c *MemoryCache) purgeEntries(keys []string) {
	c.objsMutex.Lock()
	for _, k := range keys {
		logrus.Debugf("purging %s", k)
		delete(c.objs, k)
	}
	c.objsMutex.Unlock()
}

// Store inserts a new entry into the cache
func (c *MemoryCache) Store(key string, co *ContentObject) error {
	newSize := c.memSize + co.Size()
	if newSize > c.memLimit {
		storeMetric.WithLabelValues("memory_limit").Inc()
		logrus.Debugf("memory: %d/%d", newSize, c.memLimit)
		err := c.freeMemory(co.Size())
		if err != nil {
			logrus.Debug(err)
			return err
		}
	}

	c.objsMutex.Lock()
	c.objs[key] = co
	c.memSize = newSize
	c.objsMutex.Unlock()
	storeMetric.WithLabelValues("success").Inc()
	return nil
}

// Purge deletes an item from the cache
func (c *MemoryCache) Purge(key string) error {
	c.objsMutex.Lock()
	co, ok := c.objs[key]
	if !ok {
		c.objsMutex.Unlock()
		purgeMetric.WithLabelValues("miss").Inc()
		return fmt.Errorf("object not found")
	}
	c.memSize = c.memSize - co.Size()
	delete(c.objs, key)
	c.objsMutex.Unlock()
	purgeMetric.WithLabelValues("success").Inc()
	return nil
}
