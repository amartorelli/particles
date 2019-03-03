package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/yaml.v2"

	"github.com/amartorelli/particles/pkg/cdn"
	"github.com/sirupsen/logrus"
)

func main() {
	// signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// config flags
	var confFile = flag.String("conf", "conf.yml", "path to config file")
	var loglevel = flag.String("loglevel", "info", "log level (debug/info/warn/fatal)")
	flag.Parse()

	switch *loglevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	conf, err := ioutil.ReadFile(*confFile)
	if err != nil {
		logrus.Fatal(err)
	}
	confYML := cdn.DefaultConf()
	err = yaml.Unmarshal(conf, &confYML)
	if err != nil {
		logrus.Fatalf("invalid config: %s", err)
	}

	valid, reason := confYML.IsValid()
	if !valid {
		logrus.Fatalf("invalid configuration: %s", reason)
	}

	cdn, err := cdn.NewCDN(confYML)
	if err != nil {
		logrus.Fatal(err)
	}

	exit := cdn.Start()

	// Graceful shutdown
	select {
	case sig := <-stop:
		logrus.Infof("received signnal %s, gracefully shutting down", sig.String())

	case <-exit:
		logrus.Infof("one or more handlers exited")
	}

	err = cdn.Shutdown()
	if err != nil {
		logrus.Fatalf("error terminating cdn: %s", err)
	}
}
