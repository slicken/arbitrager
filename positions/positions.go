package positions

import (
	"time"
)

// Orders holds all stop and limit orders
var Positions = make(map[int64]*Order)

// Order holds exchange order
type Order struct {
	ID   int64
	Pair string
	// Time   int64
	Time   time.Time
	Side   string
	Amount float64
	Filled float64
	Price  float64
	Stop   float64
}

// Add position to memory
func Add(id int64, symbol, side string, amount, price float64) {
	order := &Order{
		Pair:   symbol,
		ID:     id,
		Side:   side,
		Amount: amount,
		Price:  price,
		Time:   time.Now(),
	}
	Positions[id] = order
}

// Delete deletes position in memory
func Delete(id int64) bool {
	if _, ok := Positions[id]; !ok {
		return false
	}
	delete(Positions, id)
	return true
}
