package icq

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"golang.org/x/net/context"
)

type ICQClient struct {
	rnd    *rand.Rand
	aimsId string
}

const BaseUrl = "https://u.icq.net/api/v78/"

var sharedHeaders = map[string]string{
	"authority":          "u.icq.net",
	"accept":             "*/*",
	"accept-language":    "en-US,en;q=0.9,ru-RU;q=0.8,ru;q=0.7",
	"cache-control":      "no-cache",
	"origin":             "https://web.icq.com",
	"pragma":             "no-cache",
	"referer":            "https://web.icq.com/",
	"sec-ch-ua":          "\" Not A;Brand\";v=\"99\", \"Chromium\";v=\"101\", \"Google Chrome\";v=\"101\"",
	"sec-ch-ua-mobile":   "?0",
	"sec-ch-ua-platform": "\"Windows\"",
	"sec-fetch-dest":     "empty",
	"sec-fetch-mode":     "cors",
	"sec-fetch-site":     "cross-site",
	"user-agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.67 Safari/537.36",
}

func NewICQClient(token string) *ICQClient {
	s := rand.NewSource(time.Now().Unix())
	rnd := rand.New(s)
	return &ICQClient{
		rnd:    rnd,
		aimsId: token,
	}
}

func (icqInst *ICQClient) SendMessage(ctx context.Context, msg []byte, chatId string) (bool, error) {
	const requestUrl = "wim/im/sendIM"
	headers := map[string]string{
		"content-type": "application/x-www-form-urlencoded",
	}

	data := url.Values{}
	data.Set("t", chatId)                       // chatId
	data.Set("r", icqInst.genRequestId(shared)) // requestId
	data.Set("mentions", "")
	data.Set("message", string(msg))   // msg
	data.Set("f", "json")              // output format
	data.Set("aimsid", icqInst.aimsId) // token

	req, err := DoPostRequest(ctx, fmt.Sprint(BaseUrl, requestUrl), []byte(data.Encode()), headers, sharedHeaders)
	if err != nil {
		return false, fmt.Errorf("send message error: %s", err)
	}

	return true, req.Body.Close()
}

type ICQMessageEvent struct {
	Text []byte
	Err  error
}

func (icqInst *ICQClient) MessageChan(ctx context.Context, chatId string) (chan ICQMessageEvent, error) {
	const requestUrl = "bos/bos-k035b/aim/fetchEvents" //TODO Filter byChatId

	bUrl, err := url.Parse(fmt.Sprint(BaseUrl, requestUrl))
	if err != nil {
		return nil, fmt.Errorf("prepare message chan error: %s", err)
	}

	urlValues := bUrl.Query()
	urlValues.Set("timeout", "30000")
	urlValues.Set("supportedSuggestTypes", "text-smartreply%2Csticker-smartreply")
	urlValues.Set("aimsid", icqInst.aimsId)
	urlValues.Set("rnd", icqInst.genRequestId(fetch))
	decoded, err := url.QueryUnescape(urlValues.Encode()) // TODO криво :/
	if err != nil {
		return nil, fmt.Errorf("prepare message chan error: %s", err)
	}

	bUrl.RawQuery = decoded
	initFetchUrl := bUrl.String()
	msgCh := make(chan ICQMessageEvent)

	go func() {
		const maxRetry = 3
		fetchUrl := initFetchUrl
		retryCounter := 0

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if retryCounter >= maxRetry {
				msgCh <- ICQMessageEvent{
					Text: nil,
					Err:  errors.New("send fetch request error: retry count exceeded"),
				}
				close(msgCh)
				return
			}
			res, err := DoGetRequest(ctx, fetchUrl, nil, sharedHeaders)
			if err != nil {
				fetchUrl = initFetchUrl
				retryCounter++
				msgCh <- ICQMessageEvent{
					Text: nil,
					Err:  fmt.Errorf("send fetch request error: %s", err),
				}
				continue
			}

			var data fetchData
			err = json.NewDecoder(res.Body).Decode(&data)
			if err != nil {
				fetchUrl = initFetchUrl
				retryCounter++
				msgCh <- ICQMessageEvent{
					Text: nil,
					Err:  fmt.Errorf("unmarshal fetch request error: %s", err),
				}
				continue
			}
			err = res.Body.Close()
			if err != nil {
				fetchUrl = initFetchUrl
				retryCounter++
				msgCh <- ICQMessageEvent{
					Text: nil,
					Err:  fmt.Errorf("unmarshal fetch request error: %s", err),
				}
				continue
			}

			fetchUrl = data.Response.Data.FetchBaseURL
			retryCounter = 0
			for _, event := range data.Response.Data.Events {
				if event.EventData.Sn != chatId {
					continue
				}
				switch event.Type {
				case "histDlgState":
					for _, msg := range event.EventData.Messages {
						if msg.Outgoing {
							continue
						}
						msgCh <- ICQMessageEvent{
							Text: []byte(msg.Text),
							Err:  nil,
						}
					}
				}
			}
		}
	}()

	return msgCh, nil
}

type React int8

const (
	Like React = iota
	Heart
	LOL
	Embarrassed
	Cry
	Angry
)

func (icqInst *ICQClient) AddReact(ctx context.Context, react React, msgId int64, chatId string) (bool, error) {
	const requestUrl = "rapi/reaction/add"
	headers := map[string]string{
		"content-type": "application/json",
	}

	rrb := reactRequestBody{
		ReqId:  icqInst.genRequestId(shared),
		AimsId: icqInst.aimsId,
		Params: reactRequestBodyParams{
			MsgId:  msgId,
			ChatId: chatId,
			Reactions: []string{
				"👍",
				"❤️",
				"🤣",
				"😳",
				"😢",
				"😡",
			},
		},
	}

	switch react {
	case Like:
		rrb.Params.Reaction = "👍"
	case Heart:
		rrb.Params.Reaction = "❤"
	case LOL:
		rrb.Params.Reaction = "🤣"
	case Embarrassed:
		rrb.Params.Reaction = "😳"
	case Cry:
		rrb.Params.Reaction = "😢"
	case Angry:
		rrb.Params.Reaction = "😡"
	}

	reqBody, err := json.Marshal(rrb)
	if err != nil {
		return false, fmt.Errorf("add react prepare request error: %s", err)
	}

	req, err := DoPostRequest(ctx, fmt.Sprint(BaseUrl, requestUrl), reqBody, headers, sharedHeaders)
	if err != nil {
		return false, fmt.Errorf("add react send request error: %s", err)
	}

	err = req.Body.Close()
	if err != nil {
		return false, fmt.Errorf("add react error: %s", err)
	}

	return true, nil
}

type requestIdType int8

const (
	shared requestIdType = iota
	fetch
)

func (icqInst *ICQClient) genRequestId(idType requestIdType) string {
	switch idType {
	case fetch:
		return fmt.Sprintf("%d.%d", icqInst.rnd.Int63n(10000000000), icqInst.rnd.Int63n(100000))
	default:
		return fmt.Sprintf("%d-%d", icqInst.rnd.Int63n(100000), icqInst.rnd.Int63n(10000000000))
	}
}
