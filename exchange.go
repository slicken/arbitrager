package main

import (
	"errors"

	"github.com/slicken/arbitrager/config"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/exchanges/binance"
)

// LoadExchange loads an exchange by name
func LoadExchange(name string) error {
	var e exchanges.I

	switch name {
	case "binance":
		e = new(binance.Binance)
	case "kucoin":
		// e = new(kucoin.Kucoin)
	default:
		return errors.New("exchange not found")
	}

	var cfg config.ExchangeConfig

	for _, v := range config.Cfg.Exchanges {
		if v.Name == name {
			cfg = v
		}
	}
	if cfg.Name == "" {
		return errors.New("exchange config not found")
	}

	e.Setup(cfg)
	err := e.Init()
	if err == nil {
		E = e
	}

	return err
}
