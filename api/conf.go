package api

import "net"

// Conf is the configuration for the API endpoint used to purge entries from the cache
type Conf struct {
	Address  string `yaml:"address"`
	Port     int    `yaml:"port"`
	CertFile string `yaml:"cert"`
	KeyFile  string `yaml:"key"`
}

// DefaultConf returns an API configuration with some defaults
func DefaultConf() Conf {
	return Conf{Address: "0.0.0.0", Port: 7546, CertFile: "", KeyFile: ""}
}

// IsValid checks the validity of the API configuration
func (c Conf) IsValid() (bool, string) {
	valid := true
	valid = valid && c.Address != "" && net.ParseIP(c.Address) != nil
	if !valid {
		return false, "invalid API address"
	}

	valid = valid && c.Port > 0
	if !valid {
		return false, "invalid API port"
	}

	return true, ""
}
