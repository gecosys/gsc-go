package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

var mtxConfig sync.Mutex
var conf *Config

type Config struct {
	Host  string `json:"host"`
	ID    string `json:"id"`
	Token string `json:"token"`
}

func GetConfig() (*Config, error) {
	if conf != nil {
		return conf, nil
	}

	var (
		err  error
		file *os.File
		data []byte
	)

	mtxConfig.Lock()
	defer mtxConfig.Unlock()
	if conf != nil {
		return conf, nil
	}

	file, err = os.Open("./gsc-services.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err = ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	conf = new(Config)
	err = json.Unmarshal(data, conf)
	if err != nil {
		conf = nil
	}
	return conf, err
}
