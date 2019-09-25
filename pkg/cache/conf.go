package cache

const (
	// defaultTTL is the default TTL for an cache entry
	defaultTTL = 86400 // 1 day default TTL
	// defaultContentTypeRegex is the regular expression to check if the content type should be cached
	defaultContentTypeRegex = "^(image|audio|video)/.+$|^.+/javascript.*$|^text/css$"
)

// Conf is the configuration for the cache
type Conf struct {
	Type    string            `yaml:"type"`
	Options map[string]string `yaml:"options"`
}

// DefaultConf returns a cache config with some defaults
func DefaultConf() Conf {
	return Conf{Type: "memory", Options: map[string]string{"memory_limit": "10240", "ttl": "86400"}}
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
