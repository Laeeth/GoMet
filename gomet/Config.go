package gomet

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	ListenAddr string `json:"listenAddr"`

	Socks struct {
		Enable bool `json:"enable"`
		Addr string `json:"addr"`
	} `json:"socks"`

	Tunnel struct {
		ListenAddr string `json:"listenAddr"`
		Nodes[] struct {
			Type string `json:"type"`
			Host string `json:"host"`
			Username string `json:"username"`
			Password string `json:"password"`
		}`json:"nodes"`

	} `json:"tunnel"`

	Api struct {
		Enable bool `json:"enable"`
		Addr string `json:"addr"`
	} `json:"api"`

}


func LoadConfig() (Config, error) {

	log.Println("Loading configuration")

	var config Config
	configFile, err := os.Open("config/config.json")

	if err != nil {
		return config, err
	}

	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)

	if err != nil {
		return config, err
	}

	return config, nil
}
