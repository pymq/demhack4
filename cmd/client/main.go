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
	err := k.Load(file.Provider("config.json"), json.Parser())
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("error loading config: %v", err)
	}

	cfg := config.Client{}
	err = k.Unmarshal("", &cfg)
	if err != nil {
		log.Fatalf("error unmarshaling config: %v", err)
	}
	config.SetClientDefaults(&cfg)

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
	err = config.SaveConfig(cfg)
	if err != nil {
		log.Fatalf("error saving config: %v", err)
	}

	peerPubKey, err := encoding.DecodeBase64([]byte(cfg.RSAServerPublicKey))
	if err != nil {
		log.Fatalf("error decoding server public rsa key: %v", err)
	}

	proxy, err := socksproxy.NewClient(cfg.ProxyListenAddr)
	if err != nil {
		log.Fatalf("error setup proxy: %v", err)
	}
	defer func() {
		err := proxy.Close()
		if err != nil {
			log.Warnf("close proxy: %v", err)
		}
	}()

	encoder, err := encoding.NewEncoder(privateKey)
	if err != nil {
		log.Fatalf("error setup encoder: %v", err)
	}
	err = encoder.SetPeerPublicKey(peerPubKey)
	if err != nil {
		log.Fatalf("error setting server public key: %v", err)
	}

	// TODO: also pass BotRoomID
	icqClient := icq.NewICQClient(cfg.ICQ.ClientToken)
	_ = icqClient

	proxyConns := proxy.ConnsChan()
	for conn := range proxyConns {
		_ = conn
		// TODO
	}
}
