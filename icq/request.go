package icq

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var reqHTTP = &http.Client{
	Timeout: 5 * time.Minute,
	Transport: &http.Transport{
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 80 * time.Second,
	},
}

func doRequest(methode, url string, body []byte, headers, sharedHeaders map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(methode, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	for key, customHeader := range headers {
		req.Header.Set(key, customHeader)
	}
	for key, customHeader := range sharedHeaders {
		req.Header.Set(key, customHeader)
	}
	resp, err := reqHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		err = errors.New("query error: code is not 200")
		cbErr := resp.Body.Close()
		if cbErr != nil {
			err = fmt.Errorf("%s; close response body error: %s", err, cbErr)
		}
		return nil, err
	}
	return resp, nil
}

func DoGetRequest(url string, headers, sharedHeaders map[string]string) (*http.Response, error) {
	return doRequest(http.MethodGet, url, nil, headers, sharedHeaders)
}

func DoPostRequest(url string, body []byte, headers, sharedHeaders map[string]string) (*http.Response, error) {
	return doRequest(http.MethodPost, url, body, headers, sharedHeaders)
}
