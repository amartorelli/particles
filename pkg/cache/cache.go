package cache

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	validCacheTypes     = []string{"memory", "memcached"}
	errInvalidCacheType = errors.New("invalid cache type specified")
)

// Cache interface
type Cache interface {
	IsCachableContentType(contentType string) bool
	Lookup(key string) (*ContentObject, bool, error)
	Store(key string, co *ContentObject) error
	Purge(key string) error
}

// ContentObject represents a cached object
type ContentObject struct {
	content         []byte
	headers         map[string]string
	ContentType     string
	ttl             int
	cachedTimestamp int64
}

// NewCache return a new cache depending on the type and options provided
func NewCache(conf Conf) (Cache, error) {
	switch conf.Type {
	case "memory":
		c, err := NewMemoryCache(conf.Options)
		if err != nil {
			return nil, err
		}
		return c, nil
	case "memcached":
		c, err := NewMemcachedCache(conf.Options)
		if err != nil {
			return nil, err
		}
		return c, nil
	default:
		return nil, errInvalidCacheType
	}
}

// NewContentObject returns a new cache entry
func NewContentObject(data []byte, contentType string, headers map[string]string, ttl int, cachedTimestamp int64) *ContentObject {
	return &ContentObject{content: data, ContentType: contentType, headers: headers, ttl: ttl, cachedTimestamp: cachedTimestamp}
}

// Content exposes the content bytes
func (co *ContentObject) Content() []byte {
	return co.content
}

// TTL returns the size of the data
func (co *ContentObject) TTL() int {
	return co.ttl
}

// Headers returns the headers
func (co *ContentObject) Headers() map[string]string {
	return co.headers
}

// CachedTimestamp returns the timestamp of the time the object was cached
func (co *ContentObject) CachedTimestamp() int64 {
	return co.cachedTimestamp
}

// contentTypeRegex compiles a regex to be used to check cachable Content-Type
func contentTypeRegex(patterns string) (*regexp.Regexp, error) {
	if patterns != "" {
		return regexp.Compile(fmt.Sprintf("(%s)", patterns))
	}
	return regexp.Compile(defaultContentTypeRegex)
}
