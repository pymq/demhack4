package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/getlantern/systray"
	"github.com/pymq/demhack4"
	log "github.com/sirupsen/logrus"
)

var (
	//go:embed Icon.png
	appIcon []byte

	tempIconFilepath = filepath.Join(os.TempDir(), "icon.png") // TODO add appName prefix to image
)

var app *demhack4.CliApp

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	err := os.WriteFile(tempIconFilepath, appIcon, 0666)
	if err != nil {
		log.Errorf("ctray error: save app icon to temp error %v", err)
	}

	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		sig := <-quitCh
		log.Infof("received exit signal '%s'", sig)
		systray.Quit()
	}()

	err = initClientApp()
	if err != nil {
		showErrorDialog("Init proxy server error", err.Error())
		systray.Quit()
	}

	initTray()
}

func onExit() {
	if app != nil {
		app.StopProxy()
	}
	_ = os.Remove(tempIconFilepath)
}

func initClientApp() (err error) {
	defer func() {
		recovered := recover()
		if recovered != nil {
			err = fmt.Errorf("recovered panic from starting app: %v", recovered)
		}
	}()
	app = demhack4.NewCliApp()
	return nil
}
