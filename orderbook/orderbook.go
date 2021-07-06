package orderbook

import (
	"sort"
	"time"
)

const depth = 200

// Orderbook holds symbol books
var Orderbook = make(map[string]*Book)

// Book holds the fields for the orderbook Book
type Book struct {
	Name        string    `json:"pair"`
	Bids        bids      `json:"bids"`
	Asks        asks      `json:"asks"`
	LastUpdated time.Time `json:"last_updated"`
}

type orders map[float64]float64
type asks orders
type bids orders

type Item struct {
	Price  float64
	Amount float64
	Total  float64
	// LastUpdate time.Time
}

func newBook(pair string) *Book {
	var asks = make(asks, 200)
	var bids = make(bids, 200)

	book := new(Book)
	book.Name = pair
	book.Asks = asks
	book.Bids = bids
	Orderbook[pair] = book
	return book
}

// GetBook checks and returns the orderbook given an exchange name and pair
func GetBook(pair string) (*Book, bool) {
	book, ok := Orderbook[pair]
	if !ok {
		return newBook(pair), false
	}
	return book, true
}

// Pair returns Book Name
func (b *Book) Pair() string {
	return b.Name
}

func (l *asks) Add(price, amount float64) {
	if amount == 0 {
		delete(*l, price)
	} else {
		(*l)[price] = amount
	}
}

func (l *bids) Add(price, amount float64) {
	if amount == 0 {
		delete(*l, price)
	} else {
		(*l)[price] = amount
	}
}

// func (b *Book) Add(price, amount float64, bid bool) {
// 	var o orders
// 	if bid {
// 		o = orders(b.Bids)
// 	} else {
// 		o = orders(b.Asks)
// 	}

// 	if amount == 0 {
// 		delete(o, price)
// 	} else {
// 		o[price] = amount
// 	}
// 	b.LastUpdated = time.Now()
// }

// func (v orders) Get(bids bool) (items []Item) {
// 	keys := make([]float64, 0, len(v))
// 	for k := range v {
// 		keys = append(keys, k)
// 	}
// 	if bids {
// 		sort.Sort(sort.Reverse(sort.Float64Slice(keys)))
// 	} else {
// 		sort.Sort(sort.Float64Slice(keys))
// 	}

// 	var total float64 = 0
// 	for _, k := range keys {
// 		total += v[k]
// 		items = append(items, Item{k, v[k], total})
// 	}
// 	return
// }

func (v asks) Get() (items []Item) {
	keys := make([]float64, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Sort(sort.Float64Slice(keys))

	total := 0.
	for _, k := range keys {
		total += v[k]
		items = append(items, Item{k, v[k], total})
	}
	return
}

func (v bids) Get() (items []Item) {
	keys := make([]float64, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(keys)))

	total := 0.
	for _, k := range keys {
		total += v[k]
		items = append(items, Item{k, v[k], total})
	}
	return
}
