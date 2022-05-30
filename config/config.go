package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

const (
	ClientFilename = "config.json"
	ServerFilename = "config_server.json"
)

type Server struct {
	ICQBotToken string
	PrivateKey  string
}

type Client struct {
	ProxyListenAddr string
	PrivateKey      string
	ServerPublicKey string
	ICQ             struct {
		ClientToken string
		BotRoomID   string
	}
}

func SetClientDefaults(cfg *Client) {
	if cfg.ProxyListenAddr == "" {
		cfg.ProxyListenAddr = "localhost:9090"
	}
}

func SaveConfig(cfg any, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %v", err)
	}
	err = ioutil.WriteFile(path, data, 0600)
	if err != nil {
		return fmt.Errorf("save config: %v", err)
	}

	return nil
}
