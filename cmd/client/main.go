package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/libp2p/go-yamux/v3"
	"github.com/pymq/demhack4/config"
	"github.com/pymq/demhack4/encoding"
	"github.com/pymq/demhack4/icq"
	"github.com/pymq/demhack4/socksproxy"
	log "github.com/sirupsen/logrus"
)

func main() {
	k := koanf.New(".")
	err := k.Load(file.Provider(config.ClientFilename), json.Parser())
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("error loading config: %v", err)
	}

	cfg := config.Client{}
	err = k.Unmarshal("", &cfg)
	if err != nil {
		log.Fatalf("error unmarshaling config: %v", err)
	}
	config.SetClientDefaults(&cfg)

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
	err = config.SaveConfig(cfg, config.ClientFilename)
	if err != nil {
		log.Fatalf("error saving config: %v", err)
	}

	serverPubKey, err := encoding.UnmarshalPublicKey(cfg.ServerPublicKey)
	if err != nil {
		log.Fatalf("error decoding server public key: %v", err)
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

	encoder := encoding.NewEncoder(privateKey)
	err = encoder.SetPeerPublicKey([]byte(serverPubKey.String()))
	if err != nil {
		log.Fatalf("error setting server public key: %v", err)
	}

	icqClient := icq.NewICQClient(cfg.ICQ.ClientToken)

	// TODO: graceful shutdown
	ctx, _ := context.WithCancel(context.Background())

	msgCh := icqClient.MessageChan(ctx, cfg.ICQ.BotRoomID)
	rwc := icq.NewRWCClient(ctx, icqClient, msgCh, &icq.ICQEncoder{Encoder: *encoder}, encoding.MaxMessageLen, cfg.ICQ.BotRoomID)

	encKey, err := encoder.PackMessage(encoding.PublicKey, encoder.GetOwnPublicKey())
	if err != nil {
		panic(err)
	}

	err = icqClient.SendMessage(ctx, encKey, cfg.ICQ.BotRoomID)
	if err != nil {
		log.Fatalf("send public key error: %v", err)
	}

	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.EnableKeepAlive = false
	yamuxSession, err := yamux.Client(socksproxy.ConnWrapper{ReadWriteCloser: rwc}, yamuxCfg, nil)
	if err != nil {
		log.Fatalf("icq: can't start fething messages: %v", err)
		return
	}

	proxyConns := proxy.ConnsChan()
	for conn := range proxyConns {
		stream, err := yamuxSession.Open(ctx)
		if err != nil {
			log.Warnf("open yamux session error: %v", err)
			_ = conn.Close()
			continue
		}

		go bidirectionalCopy(stream, conn)
	}
}

func bidirectionalCopy(first io.ReadWriteCloser, second io.ReadWriteCloser) {
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(first, second)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(second, first)
		errCh <- err
	}()

	// Wait
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil {
			log.Warnf("proxy rwc conn error: %v", err)
		}
	}
	_ = first.Close()
	_ = second.Close()
}
