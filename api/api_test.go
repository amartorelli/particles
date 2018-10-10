package api_test

import (
	"particles/api"
	"particles/cache"
	"testing"
)

func TestPurgeHandler(t *testing.T) {
	ac := api.DefaultConf()
	cc := cache.DefaultConf()
	c, err := cache.NewCache(cc)
	if err != nil {
		t.Error(err)
	}

	a, err := api.NewAPI(ac, c)
	if err != nil {
		t.Error(err)
	}
	err = a.Start()
	if err != nil {
		t.Error(err)
	}

}
