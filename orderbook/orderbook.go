package orderbook

import (
	"sort"
	"sync"
	"time"
)

var m = &sync.Mutex{}

const DEPTH = 1000

// Orderbook holds symbol books
var Orderbook = make(map[string]*Book)

// Book holds the fields for the orderbook Book
type Book struct {
	Name        string    `json:"pair"`
	Bids        bids      `json:"bids"`
	Asks        asks      `json:"asks"`
	LastUpdated time.Time `json:"last_updated"`
	// maxDepth    int
	// sync.Mutex
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
	var asks = make(asks, DEPTH)
	var bids = make(bids, DEPTH)

	book := new(Book)
	book.Name = pair
	book.Asks = asks
	book.Bids = bids
	Orderbook[pair] = book

	return book
}

// GetBook checks and returns the orderbook given an exchange name and pair
func GetBook(pair string) (*Book, bool) {
	m.Lock()
	defer m.Unlock()

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
	m.Lock()
	defer m.Unlock()

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

// Delete book
func Delete(name string) {
	m.Lock()
	defer m.Unlock()

	delete(Orderbook, name)
}

func (l *asks) Add(price, amount float64) {
	m.Lock()
	defer m.Unlock()

	if amount == 0 {
		delete(*l, price)
	} else {
		(*l)[price] = amount
	}
}

func (l *bids) Add(price, amount float64) {
	m.Lock()
	defer m.Unlock()

	if amount == 0 {
		delete(*l, price)
	} else {
		(*l)[price] = amount
	}
}

func (b *Book) Add(price, amount float64, bid bool) {
	m.Lock()
	defer m.Unlock()

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

func (v asks) Get() (items []Item) {
	m.Lock()
	defer m.Unlock()

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
	m.Lock()
	defer m.Unlock()

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
