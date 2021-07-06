package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"sync"
)

const configJSON = "config.json"

// Cfg holds data from configJSON
var Cfg *Config
var m sync.Mutex

// Config holds config read from file
type Config struct {
	Exchanges []ExchangeConfig `json:"Exchanges"`
	Email     EmailConfig      `json:"Email"`
}

// ExchangeConfig holds all the information needed for each enabled Exchange.
type ExchangeConfig struct {
	Name     string
	Enabled  bool
	Key      string
	Secret   string
	Password string `json:",omitempty"`
}

// EmailConfig for sender
type EmailConfig struct {
	User string
	Pass string
	SMTP string
	PORT string
}

// ReadConfig file
func ReadConfig() error {
	bytes, err := ioutil.ReadFile(configJSON)
	if err != nil {
		return err
	}
	err = json.Unmarshal(bytes, &Cfg)
	if err != nil {
		return err
	}
	num := 0
	for _, e := range Cfg.Exchanges {
		if e.Name == "" {
			continue
		}
		num++
	}
	if num == 0 {
		return errors.New("no exchanges found in config file")
	}

	return nil
}

// WriteConfig to file
func WriteConfig() error {
	b, err := json.MarshalIndent(Cfg, "", "\t")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(configJSON, b, 0644)
	if err != nil {
		return err
	}

	log.Printf("successfully saved %v\n", configJSON)
	return nil
}

// ReadFile any struct
func ReadFile(target interface{}, filename string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, &target)
}

// WriteFile ..
func WriteFile(src interface{}, filename string) error {
	b, err := json.MarshalIndent(src, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, b, 0644)
}
