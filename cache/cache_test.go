package cache

import (
	"fmt"
	"testing"
)

func TestIsValid(t *testing.T) {
	tt := []struct {
		c     Conf
		valid bool
		err   error
	}{
		{Conf{Type: "invalid", Options: make(map[string]string, 0)}, false, fmt.Errorf("An invalid configuration should be invalid")},
		{Conf{Type: "memory", Options: make(map[string]string, 0)}, true, fmt.Errorf("A valid configuration should be valid")},
	}

	for _, tc := range tt {
		valid, _ := tc.c.IsValid()
		if valid != tc.valid {
			t.Error(tc.err)
		}
	}
}
