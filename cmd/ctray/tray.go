package main

import (
	"bytes"
	"context"
	"image"
	"os/exec"
	"runtime"

	ico "github.com/Kodeworks/golang-image-ico"
	"github.com/getlantern/systray"
	"github.com/ncruces/zenity"
	log "github.com/sirupsen/logrus"
)

var kdialogAvailable bool

func init() {
	_, err := exec.LookPath("kdialog")
	kdialogAvailable = err == nil
}

func getIcon() []byte {
	switch runtime.GOOS {
	case "windows":
		srcImg, _, err := image.Decode(bytes.NewReader(appIcon))
		if err != nil {
			log.Errorf("Failed to decode source image: %v", err)
			return appIcon
		}

		destBuf := new(bytes.Buffer)
		err = ico.Encode(destBuf, srcImg)
		if err != nil {
			log.Errorf("Failed to encode icon: %v", err)
			return appIcon
		}
		return destBuf.Bytes()
	default:
		return appIcon
	}
}

func initTray() {
	systray.SetIcon(getIcon())
	systray.SetTitle("Proxy")   // TODO set app name
	systray.SetTooltip("Proxy") // TODO set app name

	mStartStop := systray.AddMenuItem("Start proxy", "")
	go func() {
		started := false
		for range mStartStop.ClickedCh {
			if !started {
				err := app.StartProxy(context.Background())
				if err != nil {
					showErrorDialog("Start proxy server error", err.Error())
					continue
				}
				mStartStop.SetTitle("Stop proxy")
				started = true
			} else {
				app.StopProxy()
				started = false
				mStartStop.SetTitle("Start proxy")
			}
		}
	}()

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "")
	go func() {
		for range mQuit.ClickedCh {
			systray.Quit() // TODO add dialog?
		}
	}()
}

func showErrorDialog(title, message string) {
	var err error
	if kdialogAvailable {
		args := []string{"--error", message, "--title", title, "--icon", "dialog-error"}
		_, err = exec.Command("kdialog", args...).Output()
	} else {
		err = zenity.Error(message, zenity.Title(title), zenity.ErrorIcon)
	}
	if err != nil {
		log.Errorf("show dialog: error handling: %v", err)
	}
}
