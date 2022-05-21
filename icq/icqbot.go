package icq

import (
	"context"
	"fmt"

	botgolang "github.com/mail-ru-im/bot-golang"
)

// TODO: refactor business logic out of ICQBot
type ICQBot struct {
	Bot       *botgolang.Bot
	ctx       context.Context
	ctxCancel context.CancelFunc
	openConns map[string]*RWC
}

func NewICQBot(botToken string) (*ICQBot, error) {
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
	}
	go b.processEvents(ctx)

	return b, nil
}

func (bot *ICQBot) SendMessage(_ context.Context, msg []byte, chatId string) error {
	icqMsg := bot.Bot.NewTextMessage(chatId, string(msg))
	err := icqMsg.Send()
	if icqMsg != nil {
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

			// TODO: get public key from message, create encoder and pass to rwc

			msgCh := make(chan ICQMessageEvent, 1)
			rwc = NewRWCClient(ctx, bot, msgCh, nil, chatID)
			bot.openConns[chatID] = rwc
		}
	}
}
