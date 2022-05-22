package icq

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/libp2p/go-yamux/v3"
	botgolang "github.com/mail-ru-im/bot-golang"
	"github.com/pymq/demhack4/encoding"
	"github.com/pymq/demhack4/socksproxy"
	log "github.com/sirupsen/logrus"
)

// TODO: refactor business logic out of ICQBot, including encoder, socksproxy
type ICQBot struct {
	Bot       *botgolang.Bot
	ctx       context.Context
	ctxCancel context.CancelFunc
	openConns map[string]*RWC
	encoder   *encoding.Encoder
	proxy     *socksproxy.Server
}

func NewICQBot(botToken string, encoder *encoding.Encoder, proxy *socksproxy.Server) (*ICQBot, error) {
	bot, err := botgolang.NewBot(botToken)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &ICQBot{
		Bot:       bot,
		ctx:       ctx,
		ctxCancel: cancel,
		openConns: map[string]*RWC{},
		encoder:   encoder,
		proxy:     proxy,
	}
	go b.processEvents(ctx)

	return b, nil
}

func (bot *ICQBot) SendMessage(_ context.Context, msg []byte, chatId string) error {
	icqMsg := bot.Bot.NewTextMessage(chatId, string(msg))
	err := icqMsg.Send()
	if err != nil {
		return fmt.Errorf("bot send message error: %s", err)
	}
	return nil
}

func (bot *ICQBot) Close() error {
	bot.ctxCancel()
	return nil
}

func (bot *ICQBot) processEvents(ctx context.Context) {
	updates := bot.Bot.GetUpdatesChannel(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			if update.Type != botgolang.NEW_MESSAGE {
				continue
			}
			chatID := update.Payload.Chat.ID
			message := update.Payload.Message().Text

			rwc, exists := bot.openConns[chatID]
			if exists {
				rwc.messageChan <- ICQMessageEvent{
					Text: []byte(message),
				}
				continue
			}

			// TODO: decrypt public key
			//publicKey, flags, err := bot.encoder.UnpackMessage([]byte(message))
			//if err != nil {
			//	log.Errorf("icq: server: unpack encoded message: %v", err)
			//	continue
			//}
			decoded, err := encoding.DecodeBase64([]byte(message))
			if err != nil {
				log.Errorf("icq: server: unpack encoded message: %v", err)
				continue
			}
			flags := encoding.MessageType(binary.BigEndian.Uint64(decoded[:8]))
			publicKey := decoded[8:]

			if flags != encoding.PublicKey {
				log.Errorf("icq: server: invalid first message type from peer: '%d', should be '%d'", flags, encoding.PublicKey)
				continue
			}

			encoder := bot.encoder.Copy()
			err = encoder.SetPeerPublicKey(publicKey)
			if err != nil {
				log.Errorf("icq: server: set peer public key: %v", err)
				continue
			}

			msgCh := make(chan ICQMessageEvent, 1)
			rwc = NewRWCClient(ctx, bot, msgCh, &ICQEncoder{Encoder: *encoder}, encoding.MaxMessageLen, chatID)

			yamuxServer, err := yamux.Server(socksproxy.ConnWrapper{ReadWriteCloser: rwc}, nil, nil)
			if err != nil {
				log.Errorf("icq: server: create yamux server: %v", err)
				continue
			}
			bot.openConns[chatID] = rwc

			go func() {
				for {
					if ctx.Err() != nil {
						return
					}
					session, err := yamuxServer.Accept()
					if err != nil {
						log.Errorf("icq: server: accept yamux session: %v", err)
						return
					}

					bot.proxy.ServeConn(session)
				}
			}()
		}
	}
}
