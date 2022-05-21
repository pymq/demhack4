package icq

// fetch data

type fetchData struct {
	Response struct {
		StatusCode int    `json:"statusCode"`
		StatusText string `json:"statusText"`
		Data       struct {
			PollTime        string `json:"pollTime"`
			Ts              string `json:"ts"`
			FetchBaseURL    string `json:"fetchBaseURL"`
			FetchTimeout    int    `json:"fetchTimeout"`
			TimeToNextFetch int    `json:"timeToNextFetch"`
			Events          []struct {
				Type      string `json:"type"`
				EventData struct {
					Sn           string `json:"sn"`
					LastMsgID    string `json:"lastMsgId"`
					PatchVersion string `json:"patchVersion"`
					UnreadCnt    int    `json:"unreadCnt"`
					Yours        struct {
						LastRead string `json:"lastRead"`
					} `json:"yours"`
					Theirs struct {
						LastDelivered string `json:"lastDelivered"`
						LastRead      string `json:"lastRead"`
					} `json:"theirs"`
					Messages []struct {
						MsgID     string `json:"msgId"`
						Outgoing  bool   `json:"outgoing"`
						Time      int    `json:"time"`
						Text      string `json:"Text"`
						MediaType string `json:"mediaType"`
					} `json:"messages"`
					OlderMsgID string `json:"olderMsgId"`
					Persons    []struct {
						Sn       string   `json:"sn"`
						Friendly string   `json:"friendly"`
						Nick     string   `json:"nick"`
						Official int      `json:"official"`
						Honours  []string `json:"honours"`
					} `json:"persons"`
				} `json:"eventData"`
				SeqNum int `json:"seqNum"`
			} `json:"events"`
		} `json:"data"`
	} `json:"response"`
}

// Add React

type reactRequestBodyParams struct {
	MsgId     int64    `json:"msgId"`
	ChatId    string   `json:"chatId"`
	Reactions []string `json:"reactions"`
	Reaction  string   `json:"reaction"`
}

type reactRequestBody struct {
	ReqId  string                 `json:"reqId"`
	AimsId string                 `json:"aimsid"`
	Params reactRequestBodyParams `json:"params"`
}
