package main

import (
	"crypto/rsa"
	"fmt"
	"os"

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

	var privateKey *rsa.PrivateKey
	if len(cfg.RSAPrivateKey) == 0 {
		privateKey, _, err = encoding.GenerateKey()
		if err != nil {
			log.Fatalf("error generating rsa key: %v", err)
		}
	} else {
		privateKey, err = encoding.UnmarshalPrivateKeyWithBase64([]byte(cfg.RSAPrivateKey))
		if err != nil {
			log.Fatalf("error unmarshaling rsa key from config: %v", err)
		}
	}

	privBytes, pubBytes, err := encoding.MarshalKey(privateKey)
	if err != nil {
		log.Fatalf("error marshal rsa key: %v", err)
	}
	cfg.RSAPrivateKey = string(encoding.EncodeBase64(privBytes))
	fmt.Printf("My public key:\n%s\n", encoding.EncodeBase64(pubBytes))
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

	// TODO icq bot
	icqBot, err := icq.NewICQBot(cfg.ICQBotToken)
	if err != nil {
		log.Fatalf("error initializing icq bot: %v", err)
	}
	defer func() {
		err := icqBot.Close()
		if err != nil {
			log.Warnf("close icq bot: %v", err)
		}
	}()

	_ = icqBot.Bot
}
