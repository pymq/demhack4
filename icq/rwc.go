package icq

import (
	"context"
	"errors"
	"io"
)

type Client interface {
	SendMessage(ctx context.Context, msg []byte, chatId string) error
}

type Encoding interface {
	Encode(message []byte) ([]byte, error)
	Decode(message []byte) ([]byte, error)
}

type RWC struct {
	Client
	Encoding
	messageChan chan ICQMessageEvent
	unreadBytes []byte
	ctx         context.Context
	ctxCancel   context.CancelFunc
	chatId      string
}

func NewRWCClient(ctx context.Context, cli Client, messageChan chan ICQMessageEvent, enc Encoding, chatId string) *RWC {
	ctx, cancel := context.WithCancel(ctx)
	return &RWC{
		Client:      cli,
		Encoding:    enc,
		messageChan: messageChan,
		ctx:         ctx,
		ctxCancel:   cancel,
		chatId:      chatId,
	}
}

func (icq *RWC) Write(p []byte) (n int, err error) {
	if icq.ctx.Err() != nil {
		return 0, errors.New("write error: connection closed")
	}

	msg, err := icq.Encode(p)
	if err != nil {
		return 0, errors.New("write error: can't encode message")
	}

	err = icq.SendMessage(icq.ctx, msg, icq.chatId)
	if err != nil {
		return 0, err
	}
	return len(p), nil // TODO handle big messages
}

func (icq *RWC) Read(p []byte) (n int, err error) {
	if icq.ctx.Err() != nil {
		return 0, errors.New("read error: connection closed")
	}
	if len(p) == 0 {
		return 0, nil
	}

	if len(icq.unreadBytes) > 0 {
		n := copy(p, icq.unreadBytes)
		icq.unreadBytes = icq.unreadBytes[n:]
		return n, nil
	}

	result, closed := <-icq.messageChan
	if result.Err != nil {
		return 0, err
	}

	result.Text, err = icq.Decode(result.Text)
	if err != nil {
		return 0, errors.New("read error: can't decode message")
	}

	readBytesCounter := copy(p, result.Text)

	icq.unreadBytes = result.Text[readBytesCounter:]

	if len(icq.unreadBytes) == 0 && !closed {
		return readBytesCounter, io.EOF
	}

	return readBytesCounter, nil
}

func (icq *RWC) Close() error {
	icq.ctxCancel()
	return nil
}
