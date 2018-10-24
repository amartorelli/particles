package cdn

import (
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestBackendIsValid(t *testing.T) {
	validBackend := `name: example
domain: www.example.com
ip: 10.0.0.1
port: 80
`
	emptyNameBackend := `name:
domain: www.example.com
ip: 10.0.0.1
port: 80
`

	emptyDomainBackend := `name: example
domain:
ip: 10.0.0.1
port: 80
`

	invalidIPBackend := `name: example
domain: www.example.com
ip: 264.0.0.1
port: 80
`

	tt := []struct {
		in     string
		result bool
		errMsg string
	}{
		{validBackend, true, "backend configuration should be valid"},
		{emptyNameBackend, false, "backend configuration should be invalid because of an empty name"},
		{emptyDomainBackend, false, "backend configuration should be invalid because of an empty domain"},
		{invalidIPBackend, false, "backend configuration should be invalid because of an invalid IP"},
	}

	for _, tc := range tt {
		c := BackendConf{}
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
