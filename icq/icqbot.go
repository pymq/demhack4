package icq

import (
	"fmt"

	botgolang "github.com/mail-ru-im/bot-golang"
	"golang.org/x/net/context"
)

type ICQBot struct {
	Bot *botgolang.Bot
}

func NewICQBot(botToken string) (*ICQBot, error) {
	bot, err := botgolang.NewBot(botToken)
	if err != nil {
		return nil, err
	}
	return &ICQBot{Bot: bot}, nil
}

func (bot *ICQBot) SendMessage(_ context.Context, msg []byte, chatId string) (bool, error) {
	icqMsg := bot.Bot.NewTextMessage(chatId, string(msg))
	err := icqMsg.Send()
	if icqMsg != nil {
		return false, fmt.Errorf("bot send message error: %s", err)
	}
	return true, nil
}

func (bot *ICQBot) MessageChan(ctx context.Context, chatId string) (chan ICQMessageEvent, error) {
	updates := bot.Bot.GetUpdatesChannel(ctx)

	msgCh := make(chan ICQMessageEvent)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case update := <-updates:
				if update.Type == botgolang.NEW_MESSAGE && update.Payload.Chat.ID == chatId {
					msgCh <- ICQMessageEvent{
						Text: []byte(update.Payload.Message().Text),
						Err:  nil,
					}
				}
			}
		}
	}()

	return msgCh, nil
}
