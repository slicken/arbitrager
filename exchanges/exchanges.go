package exchanges

import (
	"errors"

	"github.com/slicken/arbitrager/client"
	"github.com/slicken/arbitrager/config"
	"github.com/slicken/arbitrager/currencie"
	"github.com/slicken/arbitrager/orderbook"
	"github.com/slicken/history"
)

// Ex stores individual exchange
type Exchange struct {
	Name     string
	Enabled  bool
	Key      string
	Password string
	Secret   string
	Auth     bool

	Pairs map[string]currencie.Pair

	*client.Requester
}

// I enforces standard functions for all Exchanges
type I interface {
	// INIT
	Init(config.ExchangeConfig) error
	// COMMON
	GetName() string
	IsEnabled() bool
	Pair(pair string) (currencie.Pair, error)
	AllPairs() map[string]currencie.Pair
	UpdatePairs() error
	// ACCOUNT
	UpdateBalance() error
	// BOOK
	GetTicker(pair string) (float64, error)
	GetAllTickers() (map[string]float64, error)
	GetKlines(pair, tf string, limit int) (history.Bars, error)
	GetOrderbook(pair string, limit int64) (*orderbook.Book, error)
	// ORDER
	SendLimit(pair, side string, amount, price float64) error
	SendMarket(pair, side string, amount float64) error
	SendCancel(pair string, id int64) error
	OrderStatus(id int64) (string, error)
	OrderFills(id int64) (float64, error)
	LastTrade(symbol string, len int64) (price float64, amount float64, err error)
	// WS
	StreamBookDiff(pair string, done <-chan bool, notify chan<- string) error  //ws:
	StreamBookDepth(pair string, done <-chan bool, notify chan<- string) error //ws:
}

// GetName returns exchangd name
func (e *Exchange) GetName() string {
	return e.Name
}

// IsEnabled is a method that returns if the current exchange is enabled
func (e *Exchange) IsEnabled() bool {
	return e.Enabled
}

// Pair returns exchange pair
func (e *Exchange) Pair(pair string) (currencie.Pair, error) {
	p := e.Pairs[pair]

	if p.Name == "" {
		return p, errors.New("not found")
	}
	return p, nil
}

// AllPairs returns all avalible pairs
func (e *Exchange) AllPairs() map[string]currencie.Pair {
	return e.Pairs
}
