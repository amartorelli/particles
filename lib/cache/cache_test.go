package cache

import (
	"testing"
)

func TestIsValid(t *testing.T) {
	tt := []struct {
		c      Conf
		valid  bool
		errMsg string
	}{
		{Conf{Type: "invalid", Options: make(map[string]string, 0)}, false, "An invalid configuration should be invalid"},
		{Conf{Type: "memory", Options: make(map[string]string, 0)}, true, "A valid configuration should be valid"},
	}

	for _, tc := range tt {
		valid, _ := tc.c.IsValid()
		if valid != tc.valid {
			t.Error(tc.errMsg)
		}
	}
}
