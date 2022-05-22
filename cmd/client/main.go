package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"io"
	"os"

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
	err = config.SaveConfig(cfg, config.ClientFilename)
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

	icqClient := icq.NewICQClient(cfg.ICQ.ClientToken)

	// TODO: graceful shutdown
	ctx, _ := context.WithCancel(context.Background())

	msgCh, err := icqClient.MessageChan(ctx, cfg.ICQ.BotRoomID)
	if err != nil {
		log.Fatalf("icq: can't start fething messages: %v", err)
	}
	rwc := icq.NewRWCClient(ctx, icqClient, msgCh, &icq.ICQEncoder{Encoder: *encoder}, encoding.MaxMessageLen, cfg.ICQ.BotRoomID)

	// TODO: encrypt public key
	//encKey, err := encoder.PackMessage(encoding.PublicKey, encoder.GetOwnPublicKey())
	//if err != nil {
	//	panic(err)
	//}
	buf := bytes.Buffer{}
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], uint64(encoding.PublicKey))
	buf.Write(data[:])
	buf.Write(encoder.GetOwnPublicKey())
	encKey := encoding.EncodeBase64(buf.Bytes())

	err = icqClient.SendMessage(ctx, encKey, cfg.ICQ.BotRoomID)
	if err != nil {
		log.Fatalf("send public key error: %v", err)
	}

	yamuxSession, err := yamux.Client(socksproxy.ConnWrapper{ReadWriteCloser: rwc}, nil, nil)
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
