package orderbook

import (
	"sort"
	"sync"
	"time"
)

var mu = &sync.Mutex{}

const DEPTH = 20

// Orderbook holds symbol books
var Orderbook = make(map[string]*Book)

// Book holds the fields for the orderbook Book
type Book struct {
	Name        string    `json:"pair"`
	Bids        bids      `json:"bids"`
	Asks        asks      `json:"asks"`
	LastUpdated time.Time `json:"last_updated"`
	pool        *sync.Pool
}

type orders map[float64]float64
type asks orders
type bids orders

type Item struct {
	Price  float64
	Amount float64
	Total  float64
}

func newBook(pair string) *Book {
	book := &Book{
		Name:        pair,
		Bids:        make(bids, DEPTH),
		Asks:        make(asks, DEPTH),
		LastUpdated: time.Time{},
		pool: &sync.Pool{New: func() interface{} {
			v := make([]float64, 0, DEPTH)
			return &v
		}},
	}
	Orderbook[pair] = book

	return book
}

// GetBook checks and returns the orderbook given an exchange name and pair
func GetBook(pair string) (*Book, bool) {
	mu.Lock()
	defer mu.Unlock()

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

// Limit depth of Bids and Asks
func (b *Book) Limit() {
	mu.Lock()
	defer mu.Unlock()

	if len(b.Asks) > DEPTH {
		for _, v := range b.Asks.Get()[DEPTH:] {
			b.Asks.Add(v.Price, 0)
		}
	}
	if len(b.Bids) > DEPTH {
		for _, v := range b.Bids.Get()[DEPTH:] {
			b.Bids.Add(v.Price, 0)
		}
	}
}

// Delete Book from map Orderbook
func Delete(name string) {
	mu.Lock()
	defer mu.Unlock()

	delete(Orderbook, name)
}

func (b *Book) Reset() {
	mu.Lock()
	defer mu.Unlock()

	b.Asks = make(asks, DEPTH)
	b.Bids = make(bids, DEPTH)
}

// Reset Asks
func (l *asks) Reset() {
	mu.Lock()
	defer mu.Unlock()

	(*l) = make(asks, DEPTH)
}

// Reset Bids
func (l *bids) Reset() {
	mu.Lock()
	defer mu.Unlock()

	(*l) = make(bids, DEPTH)
}

// Add a Ask order
func (l *asks) Add(price, amount float64) {
	mu.Lock()
	defer mu.Unlock()

	if amount == 0 {
		delete(*l, price)
	} else {
		(*l)[price] = amount
	}
}

// Add a Bid order
func (l *bids) Add(price, amount float64) {
	mu.Lock()
	defer mu.Unlock()

	if amount == 0 {
		delete(*l, price)
	} else {
		(*l)[price] = amount
	}
}

// Add Ask or Bid
func (b *Book) Add(price, amount float64, bid bool) {
	mu.Lock()
	defer mu.Unlock()

	if !bid {
		if amount == 0 {
			delete(b.Asks, price)
		} else {
			b.Asks[price] = amount
		}
	} else {
		if amount == 0 {
			delete(b.Bids, price)
		} else {
			b.Bids[price] = amount
		}
	}
	// b.Limit()
	b.LastUpdated = time.Now()
}

// Get Bids with totals (sorted)
func (v asks) Get() (items []Item) {
	mu.Lock()
	defer mu.Unlock()

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

// Get Bids with totals (sorted)
func (v bids) Get() (items []Item) {
	mu.Lock()
	defer mu.Unlock()

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
