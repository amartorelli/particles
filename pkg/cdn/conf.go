package cdn

import (
	"net"

	"github.com/amartorelli/particles/pkg/api"
	"github.com/amartorelli/particles/pkg/cache"
)

// Conf is the cdn configuration
type Conf struct {
	API   api.Conf   `yaml:"api"`
	Cache cache.Conf `yaml:"cache"`
	HTTP  HTTPConf   `yaml:"http"`
	HTTPS HTTPConf   `yaml:"https"`
}

// HTTPConf is the configuration for the http server
type HTTPConf struct {
	Address  string        `yaml:"address"`
	Port     int           `yaml:"port"`
	Backends []BackendConf `yaml:"backends"`
}

// BackendConf is the configuration for a website we cache for
type BackendConf struct {
	Name                 string `yaml:"name"`
	Domain               string `yaml:"domain"`
	IP                   string `yaml:"ip"`
	Port                 int    `yaml:"port"`
	IfModifiedValidation int    `yaml:"ifmodified_validation"`
	CertFile             string `yaml:"cert"`
	KeyFile              string `yaml:"key"`
}

// DefaultHTTPConf returns a HTTP configuration with some defaults
func DefaultHTTPConf() HTTPConf {
	return HTTPConf{Address: "0.0.0.0", Port: 80, Backends: make([]BackendConf, 0)}
}

// DefaultHTTPSConf returns a HTTPS configuration with some defaults
func DefaultHTTPSConf() HTTPConf {
	return HTTPConf{Address: "0.0.0.0", Port: 443, Backends: make([]BackendConf, 0)}
}

// DefaultConf returns a HTTP configuration with some defaults
func DefaultConf() Conf {
	return Conf{
		API:   api.DefaultConf(),
		Cache: cache.DefaultConf(),
		HTTP:  DefaultHTTPConf(),
		HTTPS: DefaultHTTPSConf(),
	}
}

// IsValid is a way of validating the configuration before starting the CDN
func (c Conf) IsValid() (bool, string) {
	// validate api config
	valid, reason := c.API.IsValid()
	if !valid {
		return false, reason
	}

	// validate cache config
	valid, reason = c.Cache.IsValid()
	if !valid {
		return false, reason
	}

	// validate HTTP config
	valid, reason = c.HTTP.IsValid()
	if !valid {
		return false, reason
	}

	// validate HTTPS config
	valid, reason = c.HTTPS.IsValid()
	if !valid {
		return false, reason
	}

	return true, ""
}

// IsValid checks the validity of a HTTP conf
func (hc HTTPConf) IsValid() (bool, string) {
	valid := true
	valid = valid && hc.Address != "" && net.ParseIP(hc.Address) != nil
	if !valid {
		return false, "invalid HTTP/HTTPS address"
	}

	valid = valid && hc.Port > 0
	if !valid {
		return false, "invalid HTTP/HTTPS port"
	}

	for _, b := range hc.Backends {
		bval, reason := b.IsValid()
		valid = valid && bval
		if !valid {
			return false, reason
		}
	}

	return true, ""
}

// IsValid checks the validity of a backend config
func (bc BackendConf) IsValid() (bool, string) {
	valid := bc.Name != "" && bc.Domain != "" && net.ParseIP(bc.IP) != nil
	if !valid {
		return false, "invalid HTTP/HTTPS backend"
	}
	return true, ""
}
