package client

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

type CliApp struct {
	cfg       config.Client
	encoder   *encoding.Encoder
	ctxCancel context.CancelFunc
}

func NewCliApp() *CliApp {
	k := koanf.New(".")
	err := k.Load(file.Provider(config.ClientFilename), json.Parser())
	if err != nil && !os.IsNotExist(err) {
		log.Panicf("error loading config: %v", err)
	}

	cfg := config.Client{}
	err = k.Unmarshal("", &cfg)
	if err != nil {
		log.Panicf("error unmarshaling config: %v", err)
	}
	config.SetClientDefaults(&cfg)

	var privateKey *age.X25519Identity
	if len(cfg.PrivateKey) == 0 {
		privateKey, err = encoding.GenerateKey()
		if err != nil {
			log.Panicf("error generating private key: %v", err)
		}
	} else {
		privateKey, err = encoding.UnmarshalPrivateKey(cfg.PrivateKey)
		if err != nil {
			log.Panicf("error unmarshaling private key from config: %v", err)
		}
	}

	cfg.PrivateKey = privateKey.String()
	fmt.Printf("My public key:\n%s\n", privateKey.Recipient().String())
	// saving new values from defaults, generated private key
	err = config.SaveConfig(cfg, config.ClientFilename)
	if err != nil {
		log.Panicf("error saving config: %v", err)
	}

	serverPubKey, err := encoding.UnmarshalPublicKey(cfg.ServerPublicKey)
	if err != nil {
		log.Panicf("error decoding server public key: %v", err)
	}

	encoder := encoding.NewEncoder(privateKey)
	err = encoder.SetPeerPublicKey([]byte(serverPubKey.String()))
	if err != nil {
		log.Panicf("error setting server public key: %v", err)
	}

	return &CliApp{cfg: cfg, encoder: encoder}
}

func (app *CliApp) StartProxy(ctx context.Context) error {
	ctx, app.ctxCancel = context.WithCancel(ctx)

	icqClient := icq.NewICQClient(app.cfg.ICQ.ClientToken)
	msgCh := icqClient.MessageChan(ctx, app.cfg.ICQ.BotRoomID)
	rwc := icq.NewRWCClient(ctx, icqClient, msgCh, &icq.ICQEncoder{Encoder: *app.encoder}, encoding.MaxMessageLen, app.cfg.ICQ.BotRoomID)

	encKey, err := app.encoder.PackMessage(encoding.PublicKey, app.encoder.GetOwnPublicKey())
	if err != nil {
		return fmt.Errorf("pack message error: %v", err)
	}

	err = icqClient.SendMessage(ctx, encKey, app.cfg.ICQ.BotRoomID)
	if err != nil {
		return fmt.Errorf("send public key error: %v", err)
	}

	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.EnableKeepAlive = false
	yamuxSession, err := yamux.Client(socksproxy.ConnWrapper{ReadWriteCloser: rwc}, yamuxCfg, nil)
	if err != nil {
		return fmt.Errorf("init yamux client connection error: %v", err)
	}

	proxy, err := socksproxy.NewClient(app.cfg.ProxyListenAddr)
	if err != nil {
		return fmt.Errorf("setup proxy error: %v", err)
	}

	proxyConns := proxy.ConnsChan()
	go func() {
		for {
			select {
			case <-ctx.Done():
				err = yamuxSession.Close()
				if err != nil {
					log.Warnf("close yamux session error: %v", err)
				}
				err := proxy.Close()
				if err != nil {
					log.Warnf("close proxy error: %v", err)
				}
				return
			case conn := <-proxyConns:
				stream, err := yamuxSession.Open(ctx)
				if err != nil {
					log.Warnf("open yamux session error: %v", err)
					err = conn.Close()
					if err != nil {
						log.Warnf("close proxy connection error: %v", err)
					}
				} else {
					go bidirectionalCopy(stream, conn)
				}
			}
		}
	}()
	return nil
}

func (app *CliApp) StopProxy() {
	if app.ctxCancel != nil {
		app.ctxCancel()
		app.ctxCancel = nil
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
