package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pymq/demhack4/cmd/iternal"
	log "github.com/sirupsen/logrus"
)

func main() {
	app := iternal.NewCliApp()
	err := app.StartProxy(context.Background())
	if err != nil {
		log.Panicf("start proxy error: %v", err)
	}
	defer func() {
		app.StopProxy()
	}()

	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-quitCh
	log.Infof("received exit signal '%s'", sig)
}
