package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"filippo.io/age"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/pymq/demhack4/config"
	"github.com/pymq/demhack4/encoding"
	"github.com/pymq/demhack4/icq"
	"github.com/pymq/demhack4/socksproxy"
	log "github.com/sirupsen/logrus"
)

func main() {
	k := koanf.New(".")
	err := k.Load(file.Provider(config.ServerFilename), json.Parser())
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("error loading config: %v", err)
	}

	cfg := config.Server{}
	err = k.Unmarshal("", &cfg)
	if err != nil {
		log.Fatalf("error unmarshaling config: %v", err)
	}

	var privateKey *age.X25519Identity
	if len(cfg.PrivateKey) == 0 {
		privateKey, err = encoding.GenerateKey()
		if err != nil {
			log.Fatalf("error generating private key: %v", err)
		}
	} else {
		privateKey, err = encoding.UnmarshalPrivateKey(cfg.PrivateKey)
		if err != nil {
			log.Fatalf("error unmarshaling private key from config: %v", err)
		}
	}

	cfg.PrivateKey = privateKey.String()
	fmt.Printf("My public key:\n%s\n", privateKey.Recipient().String())
	// saving new values from defaults, generated private key
	err = config.SaveConfig(cfg, config.ServerFilename)
	if err != nil {
		log.Fatalf("error saving config: %v", err)
	}

	proxy := socksproxy.NewServer()
	defer func() {
		err := proxy.Close()
		if err != nil {
			log.Warnf("close proxy: %v", err)
		}
	}()

	encoder := encoding.NewEncoder(privateKey)

	icqBot, err := icq.NewICQBot(cfg.ICQBotToken, encoder, proxy)
	if err != nil {
		log.Fatalf("error initializing icq bot: %v", err)
	}
	defer func() {
		err := icqBot.Close()
		if err != nil {
			log.Warnf("close icq bot: %v", err)
		}
	}()

	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-quitCh
	log.Infof("received exit signal '%s'", sig)
}
