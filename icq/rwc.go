package icq

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/pymq/demhack4/encoding"
)

type Client interface {
	SendMessage(ctx context.Context, msg []byte, chatId string) error
}

type Encoding interface {
	Encode(message []byte) ([]byte, error)
	Decode(message []byte) ([]byte, encoding.MessageType, error)
	SetPeerPublicKey(publicKey []byte) error
}

type RWC struct {
	cli          Client
	encoding     Encoding
	messageChan  chan ICQMessageEvent
	unreadBytes  []byte
	ctx          context.Context
	ctxCancel    context.CancelFunc
	chatId       string
	messageLimit int
}

func NewRWCClient(ctx context.Context, cli Client, messageChan chan ICQMessageEvent, enc Encoding, messageLimit int, chatId string) *RWC {
	ctx, cancel := context.WithCancel(ctx)
	return &RWC{
		cli:          cli,
		encoding:     enc,
		messageChan:  messageChan,
		ctx:          ctx,
		ctxCancel:    cancel,
		chatId:       chatId,
		messageLimit: messageLimit,
	}
}

func (icq *RWC) Write(p []byte) (n int, err error) {
	if icq.ctx.Err() != nil {
		return 0, errors.New("write error: connection closed")
	}

	for len(p) != 0 {
		chunk := p
		if len(p) > icq.messageLimit {
			chunk = p[:icq.messageLimit]
		}

		msg, err := icq.encoding.Encode(chunk)
		if err != nil {
			return 0, fmt.Errorf("read: can't encode message: %v", err)
		}
		err = icq.cli.SendMessage(icq.ctx, msg, icq.chatId)
		if err != nil {
			return n, err
		}

		n += len(chunk)
		p = p[len(chunk):]
	}

	return n, nil
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

	result, open := <-icq.messageChan
	if result.Err != nil {
		return 0, result.Err
	} else if !open && len(result.Text) == 0 {
		return 0, io.EOF
	}

	var msgType encoding.MessageType
	result.Text, msgType, err = icq.encoding.Decode(result.Text)
	if err != nil {
		return 0, fmt.Errorf("read: can't decode message: %v", err)
	}
	if msgType == encoding.PublicKey {
		err = icq.encoding.SetPeerPublicKey(result.Text)
		if err != nil {
			return 0, fmt.Errorf("read: set public key: %v", err)
		}
		return 0, nil
	}

	readBytesCounter := copy(p, result.Text)

	icq.unreadBytes = result.Text[readBytesCounter:]

	if len(icq.unreadBytes) == 0 && !open {
		return readBytesCounter, io.EOF
	}

	return readBytesCounter, nil
}

func (icq *RWC) Close() error {
	icq.ctxCancel()
	return nil
}
