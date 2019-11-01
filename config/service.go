package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

var onceConfig sync.Once
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
	onceConfig.Do(func() {
		if conf != nil {
			return
		}

		file, err = os.Open("./gsc-services.json")
		if err != nil {
			return
		}
		defer file.Close()

		data, err = ioutil.ReadAll(file)
		if err != nil {
			return
		}

		conf = new(Config)
		err = json.Unmarshal(data, conf)
	})
	return conf, err
}
