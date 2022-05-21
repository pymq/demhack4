package icq

import (
	"context"
	"errors"
	"io"
)

type Client interface {
	SendMessage(ctx context.Context, msg []byte, chatId string) (bool, error)
	MessageChan(ctx context.Context, chatId string) (chan ICQMessageEvent, error)
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

func NewRWCClient(ctx context.Context, cli Client, enc Encoding, chatId string) (*RWC, error) {
	ctx, cancel := context.WithCancel(ctx)

	msgCh, err := cli.MessageChan(ctx, chatId)
	if err != nil {
		cancel()
		return nil, err
	}

	return &RWC{
		Client:      cli,
		Encoding:    enc,
		messageChan: msgCh,
		ctx:         ctx,
		ctxCancel:   cancel,
		chatId:      chatId,
	}, nil
}

func (icq *RWC) Write(p []byte) (n int, err error) {
	if icq.ctx.Err() != nil {
		return 0, errors.New("write error: connection closed")
	}

	msg, err := icq.Encode(p)
	if err != nil {
		return 0, errors.New("write error: can't encode message")
	}

	_, err = icq.SendMessage(icq.ctx, msg, icq.chatId)
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

	relocatedSliceBytes := func(from, to []byte, counter int) {
		for i := 0; i < len(to) && len(from) > 0; i++ {
			to[i] = from[0]
			from = from[1:]
			counter++
		}
	}

	readBytesCounter := 0

	if len(icq.unreadBytes) > 0 {
		relocatedSliceBytes(icq.unreadBytes, p, readBytesCounter)
		return readBytesCounter, nil
	}

	result, ok := <-icq.messageChan
	if result.Err != nil {
		return 0, err
	}

	result.Text, err = icq.Decode(result.Text)
	if err != nil {
		return 0, errors.New("read error: can't decode message")
	}
	
	relocatedSliceBytes(result.Text, p, readBytesCounter)

	icq.unreadBytes = append(icq.unreadBytes, result.Text...)

	if len(icq.unreadBytes) == 0 && !ok {
		return readBytesCounter, io.EOF
	}

	return readBytesCounter, nil
}

func (icq *RWC) Close() error {
	icq.ctxCancel()
	return nil
}
