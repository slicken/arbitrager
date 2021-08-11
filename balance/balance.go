package balance

import (
	"time"
)

// Balances holds all exchange balances
var Balances = make(map[string]*Balance)

// Balance stores currencie balance
type Balance struct {
	Asset       string
	Free        float64
	Locked      float64
	LastUpdated time.Time `json:"last_updated"`
}

func Get(asset string) *Balance {
	balance, ok := Balances[asset]
	if !ok {
		return nil
	}
	return balance
}

// Update the Orderbook
func (b *Balance) Update(curr string) {
	Balances[curr] = b
}
