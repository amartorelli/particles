package api

import (
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestIsValid(t *testing.T) {
	// validConf is a perfectly valid API configuration
	validConf := `address: 192.168.1.10
port: 1234
cert_file: /tmp/test.crt
key_file: /tmp/test.key
`

	// invalidAddress is an invalid configuration because the address isn't a valid IP
	invalidAddress := `address: 192.168.1
port: 1234
cert_file: /tmp/test.crt
key_file: /tmp/test.key
`

	// invalidPort is an invalid configuration because the port is a negative number
	invalidPort := `address: 192.168.1.10
port: -1
cert_file: /tmp/test.crt
key_file: /tmp/test.key
`

	tt := []struct {
		in     string
		result bool
		errMsg string
	}{
		{validConf, true, "configuration should be valid"},
		{invalidAddress, false, "configuration should be invalid because the address isn't an valid IP"},
		{invalidPort, false, "configuration should be invalid because the port isn't a number"},
	}

	for _, tc := range tt {
		c := Conf{}
		err := yaml.Unmarshal([]byte(tc.in), &c)
		if err != nil {
			t.Errorf("invalid config: %s", err)
		}

		valid, _ := c.IsValid()
		if tc.result != valid {
			t.Error(tc.errMsg)
		}
	}
}
