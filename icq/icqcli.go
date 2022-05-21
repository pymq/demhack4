package icq

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"time"
)

type ICQClient struct {
	rnd    *rand.Rand
	aimsId string
}

const BotRoomId = "70001" // TODO replace to real bot id
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

func (icqInst *ICQClient) SendMessage(msg []byte) (bool, error) {
	const requestUrl = "wim/im/sendIM"
	headers := map[string]string{
		"content-type": "application/x-www-form-urlencoded",
	}

	data := url.Values{}
	data.Set("t", BotRoomId)                    // chatId
	data.Set("r", icqInst.genRequestId(shared)) // requestId
	data.Set("mentions", "")
	data.Set("message", string(msg))   // msg
	data.Set("f", "json")              // output format
	data.Set("aimsid", icqInst.aimsId) // token

	req, err := DoPostRequest(fmt.Sprint(BaseUrl, requestUrl), []byte(data.Encode()), headers, sharedHeaders)
	if err != nil {
		return false, fmt.Errorf("send message error: %s", err)
	}

	return true, req.Body.Close()
}

type GetMessageChanMessage struct {
	Text []byte
	Err  error
}

func (icqInst *ICQClient) GetMessageChan() (chan GetMessageChanMessage, error) {
	const requestUrl = "bos/bos-k035b/aim/fetchEvents"

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
	msgCh := make(chan GetMessageChanMessage)

	go func() {
		const maxRetry = 3
		fetchUrl := initFetchUrl
		retryCounter := 0
		for {
			if retryCounter >= maxRetry {
				msgCh <- GetMessageChanMessage{
					Text: nil,
					Err:  errors.New("send fetch request error: retry count exceeded"),
				}
				close(msgCh)
				break
			}
			res, err := DoGetRequest(fetchUrl, nil, sharedHeaders)
			if err != nil {
				fetchUrl = initFetchUrl
				retryCounter++
				msgCh <- GetMessageChanMessage{
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
				msgCh <- GetMessageChanMessage{
					Text: nil,
					Err:  fmt.Errorf("unmarshal fetch request error: %s", err),
				}
				continue
			}
			err = res.Body.Close()
			if err != nil {
				fetchUrl = initFetchUrl
				retryCounter++
				msgCh <- GetMessageChanMessage{
					Text: nil,
					Err:  fmt.Errorf("unmarshal fetch request error: %s", err),
				}
				continue
			}

			fetchUrl = data.Response.Data.FetchBaseURL
			retryCounter = 0
			for _, event := range data.Response.Data.Events {
				switch event.Type {
				case "histDlgState":
					for _, msg := range event.EventData.Messages {
						if msg.Outgoing {
							continue
						}
						msgCh <- GetMessageChanMessage{
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

func (icqInst *ICQClient) AddReact(react React, msgId int64) (bool, error) {
	const requestUrl = "rapi/reaction/add"
	headers := map[string]string{
		"content-type": "application/json",
	}

	rrb := reactRequestBody{
		ReqId:  icqInst.genRequestId(shared),
		AimsId: icqInst.aimsId,
		Params: reactRequestBodyParams{
			MsgId:  msgId,
			ChatId: BotRoomId, // TODO should be changeable????
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

	req, err := DoPostRequest(fmt.Sprint(BaseUrl, requestUrl), reqBody, headers, sharedHeaders)
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

type ICQClientRWC struct {
	*ICQClient
	messageChan chan GetMessageChanMessage
	// TODO Сюда же можно функцию для стеганографии текста можно впихнуть и шифрования/дешифрования, если надо менять их
}

func (icqInst *ICQClient) NewICQClientRWC() (*ICQClientRWC, error) {
	msgCh, err := icqInst.GetMessageChan()
	if err != nil {
		return nil, err
	}
	return &ICQClientRWC{
		ICQClient:   icqInst,
		messageChan: msgCh,
	}, nil
}

func (icq *ICQClientRWC) Write(p []byte) (n int, err error) {
	_, err = icq.SendMessage(p)
	if err != nil {
		return 0, err
	}
	return len(p), nil // TODO handle big messages
}

func (icq *ICQClientRWC) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	result, ok := <-icq.messageChan
	if result.Err != nil {
		return 0, err
	}
	if !ok {

	}
	return 0, nil
}

func (icq *ICQClientRWC) Close() {

}
