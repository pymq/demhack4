package icq

import (
	"context"
	"errors"
	"io"
)

type client interface {
	SendMessage(ctx context.Context, msg []byte, chatId string) (bool, error)
	MessageChan(ctx context.Context, chatId string) (chan ICQMessageEvent, error)
}

type RWC struct {
	client
	messageChan chan ICQMessageEvent
	unreadBytes []byte
	ctx         context.Context
	ctxCancel   context.CancelFunc
	chatId      string
	// TODO Сюда же можно функцию для стеганографии текста можно впихнуть и шифрования/дешифрования, если надо менять их
}

func NewRWCClient(ctx context.Context, cli client, chatId string) (*RWC, error) {
	ctx, cancel := context.WithCancel(ctx)

	msgCh, err := cli.MessageChan(ctx, chatId)
	if err != nil {
		cancel()
		return nil, err
	}

	return &RWC{
		client:      cli,
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
	_, err = icq.SendMessage(icq.ctx, p, icq.chatId)
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
