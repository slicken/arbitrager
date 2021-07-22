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
		e = &binance.Binance{Debug: debug}
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

	if err := e.Init(cfg); err != nil {
		return err
	}
	E = e

	return nil
}
