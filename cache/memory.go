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
	// defaultForcePurge enables the deletion of random items if no space can be freed up
	defaultForcePurge = true
)

// MemoryCache represents a cache object
type MemoryCache struct {
	objs              map[string]*MemoryItem
	contentTypeRegexp *regexp.Regexp
	defaultTTL        int
	objsMutex         sync.RWMutex
	memLimit          int
	memSize           int
	hits              int
	misses            int
	forcePurge        bool
}

// MemoryCacheConfig is how the configuration for the memory cache is represented in the config file
type MemoryCacheConfig struct {
	MemoryLimit int      `yaml:"memory_limit"` // mandatory, how much memory in bytes to use
	TTL         int      `yaml:"ttl"`          // optional, how long each entry is cached for
	Patterns    []string `yaml:"patterns"`     // optional, content-type patterns
	ForcePurge  bool     `yaml:"force_purge"`  // optional, delete random items if memory can't be freed up
}

// MemoryItem is the structure used for the in-memory cache
type MemoryItem struct {
	co          *ContentObject
	timestamp   time.Time
	ttl         int
	expiration  time.Time
	contentSize int
	hits        int
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

	// force purge
	fp := defaultForcePurge
	v, ok = options["force_purge"]
	if ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("error parsing force_purge: %s", err)
		}
		fp = b
	}

	return &MemoryCache{objs: make(map[string]*MemoryItem), contentTypeRegexp: regex, objsMutex: sync.RWMutex{}, memLimit: ml, forcePurge: fp}, nil
}

// IsCachableContentType returns true in case the content type is one that can be cached
func (c *MemoryCache) IsCachableContentType(contentType string) bool {
	return c.contentTypeRegexp.MatchString(contentType)
}

// Lookup returns the content if present and a boolean to represent if it's been found
func (c *MemoryCache) Lookup(key string) (*ContentObject, bool, error) {
	start := time.Now()
	defer lookupDuration.WithLabelValues("memory").Observe(time.Since(start).Seconds())

	c.objsMutex.RLock()
	mi, found := c.objs[key]
	if found {
		// if the entry has expired, don't return it and delete it
		if time.Now().After(mi.Expiration()) {
			lookupMetric.WithLabelValues("memory", "miss").Inc()
			c.misses++
			delete(c.objs, key)
			c.objsMutex.RUnlock()
			logrus.Debugf("item %s is expired", key)
			return nil, false, fmt.Errorf("expired entry")
		}
		lookupMetric.WithLabelValues("memory", "hit").Inc()
		mi.hits++
		c.hits++
		c.objsMutex.RUnlock()
		logrus.Debugf("successfully looked up %s", key)
		return mi.co, found, nil
	}
	lookupMetric.WithLabelValues("memory", "miss").Inc()
	c.misses++
	c.objsMutex.RUnlock()

	logrus.Debugf("item %s not found", key)
	return nil, found, nil
}

// freeMemory frees up some space in the hash map to at least fit a new object of size bytes
// By default it deletes entries that have been hit less than 10% of the current total hits received
// by the cache.
// freeMemory is not accurate, to make sure we don't block requests, we avoid locking the map during the check.
func (c *MemoryCache) freeMemory(size int) error {
	logrus.Debugf("freeing up memory to allocate %d bytes", size)
	var tbd []string
	var fs int
	// free memory by removing an entry that has been hit less than
	// 10% of total hits
	i := 10
	done := false

	for i <= 50 && !done {
		percentHits := i * c.hits / 100

		for k, co := range c.objs {
			// delete any expired item or items with a low percentage of hit rate
			if time.Now().After(co.Expiration()) || co.hits < percentHits {
				tbd = append(tbd, k)
				fs = fs + co.Size()
			}

			if fs+c.memSize > size {
				done = true
				break
			}
		}
		i++
	}

	// if we couldn't free enough space and the force purge is set, delete random items
	if len(tbd) == 0 && c.forcePurge {
		for k, co := range c.objs {
			tbd = append(tbd, k)
			fs = fs + co.Size()

			if fs+c.memSize > size {
				done = true
				break
			}
		}
	}

	c.purgeEntries(tbd)
	if fs+c.memSize < size {
		return fmt.Errorf("unable to free enough memory (%d/%d)", fs, size)
	}
	logrus.Debugf("successfully freed memory %d", fs)
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
	start := time.Now()
	defer storeDuration.WithLabelValues("memory").Observe(time.Since(start).Seconds())

	size := len(co.Content())

	if size > c.memLimit {
		return fmt.Errorf("item %s can't fit in memory", key)
	}
	newSize := c.memSize + size
	if newSize > c.memLimit {
		storeMetric.WithLabelValues("memory", "memory_limit").Inc()
		logrus.Debugf("memory: %d/%d", newSize, c.memLimit)
		err := c.freeMemory(size)
		if err != nil {
			logrus.Debugf("error storing item %s: %s", key, err)
			return err
		}
	}

	now := time.Now()
	ttl := co.TTL()
	if co.TTL() == 0 {
		ttl = defaultTTL
		co.ttl = defaultTTL
	}

	mi := &MemoryItem{co: co, timestamp: now, ttl: ttl, expiration: now.Add(time.Duration(ttl) * time.Second), contentSize: size}
	c.objsMutex.Lock()
	c.objs[key] = mi
	c.memSize = newSize
	c.objsMutex.Unlock()

	logrus.Debugf("successfully stored item for %s", key)
	storeMetric.WithLabelValues("memory", "success").Inc()
	return nil
}

// Purge deletes an item from the cache
func (c *MemoryCache) Purge(key string) error {
	start := time.Now()
	defer purgeDuration.WithLabelValues("memory").Observe(time.Since(start).Seconds())

	c.objsMutex.Lock()
	co, ok := c.objs[key]
	if !ok {
		c.objsMutex.Unlock()
		purgeMetric.WithLabelValues("memory", "miss").Inc()
		return fmt.Errorf("object not found")
	}
	c.memSize = c.memSize - co.Size()
	delete(c.objs, key)
	c.objsMutex.Unlock()
	logrus.Debugf("successfully purged item %s", key)
	purgeMetric.WithLabelValues("memory", "success").Inc()
	return nil
}

// Expiration returns the expiration time for an entry
func (co *MemoryItem) Expiration() time.Time {
	return co.expiration
}

// Size returns the size of the data
func (co *MemoryItem) Size() int {
	return co.contentSize
}
