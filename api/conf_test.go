package api_test

import (
	"fmt"
	"particles/api"
	"testing"

	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

var validConf = `address: 192.168.1.10
port: 1234
cert_file: /tmp/test.crt
key_file: /tmp/test.key
`

var invalidAddress = `address: 192.168.1
port: 1234
cert_file: /tmp/test.crt
key_file: /tmp/test.key
`

var invalidPort = `address: 192.168.1.10
port: -1
cert_file: /tmp/test.crt
key_file: /tmp/test.key
`

func TestIsValid(t *testing.T) {
	tests := []struct {
		in     string
		result bool
		err    error
	}{
		{in: validConf, result: true, err: fmt.Errorf("configuration should be valid")},
		{in: invalidAddress, result: false, err: fmt.Errorf("configuration should be invalid because the address isn't an valid IP")},
		{in: invalidPort, result: false, err: fmt.Errorf("configuration should be invalid because the port isn't a number")},
	}

	for _, test := range tests {
		c := api.Conf{}
		err := yaml.Unmarshal([]byte(test.in), &c)
		if err != nil {
			logrus.Fatalf("invalid config: %s", err)
		}

		valid, _ := c.IsValid()
		if test.result != valid {
			t.Errorf(test.err.Error())
		}
	}
}
