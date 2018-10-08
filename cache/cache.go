package cache

import (
	"fmt"
	"regexp"
	"time"
)

const (
	// defaultTTL is the default TTL for an cache entry
	defaultTTL = 86400 // 1 day default TTL
	// defaultContentTypeRegex is the regular expression to check if the content type should be cached
	defaultContentTypeRegex = "(^(image|audio|video)/.+$|^.+/javascript.*$|^text/css$)"
)

var validCacheTypes = []string{"memory", "memcached"}

// Conf is the configuration for the cache
type Conf struct {
	Type    string            `yaml:"type"`
	Options map[string]string `yaml:"options"`
}

// DefaultConf returns a cache config with some defaults
func DefaultConf() Conf {
	return Conf{Type: "memory", Options: map[string]string{"memory_limit": "10240", "ttl": "86400"}}
}

// Cache interface
type Cache interface {
	IsCachableContentType(key string) bool
	Lookup(key string) (*ContentObject, bool, error)
	Store(key string, co *ContentObject) error
	Purge(key string) error
}

// ContentObject represents a cached object
type ContentObject struct {
	content     []byte
	headers     map[string]string
	ContentType string
	timestamp   time.Time
	ttl         int
	expiration  time.Time
	contentSize int
	hits        int
}

// IsValid checks the cache configuration is valid
func (c Conf) IsValid() (bool, string) {
	for _, t := range validCacheTypes {
		if c.Type == t {
			return true, ""
		}
	}
	return false, "invalid cache type"
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
		return nil, fmt.Errorf("no cache of type %s", conf.Type)
	}
}

// NewContentObject returns a new cache entry
func NewContentObject(data []byte, contentType string, headers map[string]string, ttl int) *ContentObject {
	now := time.Now()
	if ttl == 0 {
		ttl = defaultTTL
	}
	return &ContentObject{content: data, ContentType: contentType, headers: headers, timestamp: now, ttl: ttl, expiration: now.Add(time.Duration(ttl) * time.Second), contentSize: len(data)}
}

// Content exposes the content bytes
func (co *ContentObject) Content() []byte {
	return co.content
}

// Size returns the size of the data
func (co *ContentObject) Size() int {
	return co.contentSize
}

// TTL returns the size of the data
func (co *ContentObject) TTL() int {
	return co.ttl
}

// Headers returns the headers
func (co *ContentObject) Headers() map[string]string {
	return co.headers
}

// Expiration returns the expiration time for an entry
func (co *ContentObject) Expiration() time.Time {
	return co.expiration
}

func contentTypeRegex(patterns string) (*regexp.Regexp, error) {
	if patterns != "" {
		return regexp.Compile(fmt.Sprintf("(%s)", patterns))
	}
	return regexp.Compile(defaultContentTypeRegex)
}
