package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/slicken/arbitrager/currencie"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/orderbook"
)

type action byte

const (
	buy  action = 0
	sell action = 1
)

var actions = map[action]string{
	0: "buy",
	1: "sell",
}

type route [3]action

type set struct {
	asset string
	route route
	a     currencie.Pair
	b     currencie.Pair
	c     currencie.Pair
}

type sets []set

type Arbitrage interface {
	Sets(string) sets
}

type bbs struct{ route }
type bss struct{ route }
type sbb struct{ route }
type ssb struct{ route }

var arbs = []Arbitrage{
	&bbs{route{buy, buy, sell}},
	&bss{route{buy, sell, sell}},
	&sbb{route{sell, buy, buy}},
	&ssb{route{sell, sell, sell}},
}

var SetsMap = make(map[currencie.Pair]sets)

func (s sets) List() (list []string) {
	var m = make(map[string]currencie.Pair)

	for _, v := range s {
		m[v.a.Name] = v.a
		m[v.b.Name] = v.b
		m[v.c.Name] = v.c
	}
	for k := range m {
		list = append(list, k)
	}
	return
}

func pairsByQuote(e exchanges.I, quote, except string) (pairs []currencie.Pair) {
	quote = strings.ToUpper(quote)
	for _, p := range e.AllPairs() {
		if !p.Enabled {
			continue
		}
		if quote == p.Quote && except != p.Base {
			pairs = append(pairs, p)
		}
	}
	return
}

func pairsByBase(e exchanges.I, base, except string) (pairs []currencie.Pair) {
	base = strings.ToUpper(base)
	for _, p := range e.AllPairs() {
		if !p.Enabled {
			continue
		}
		if base == p.Base && except != p.Quote {
			pairs = append(pairs, p)
		}
	}
	return
}

func (s set) calcProfit(amount float64) bool {
	var pair = [3]string{s.a.Name, s.b.Name, s.c.Name}
	var price [3]float64

	for i, _action := range s.route {
		book, _ := orderbook.GetBook(pair[i])

		// get the right prices
		var depth []orderbook.Item
		switch _action {
		case buy:
			depth = book.Asks.Get()
		case sell:
			depth = book.Bids.Get()
		}

		for _, v := range depth {
			if v.Total >= amount {
				price[i] = v.Price
				break
			}
		}

		if price[i] == 0 {
			return false
		}
	}

	// for bbs
	profit := amount/price[0]/price[1]*price[2] - amount
	// TODO: bss / sbb / ssb

	if profit > 0. || debug {
		log.Printf("%s:: %-12f %8s %-12s %-12f %12s %-12s %-12f %12s %-12s %-12f\n", s.asset, profit, actions[s.route[0]], pair[0], price[0], actions[s.route[1]], pair[1], price[1], actions[s.route[2]], pair[2], price[2])
		return true
	}

	// TODO: when profit .. check what biggest profit amout we can get vs diffrent depths in books

	return false
}

func (s *bbs) Sets(curr string) (Sets sets) {
	curr = strings.ToUpper(curr)

	for i, a := range pairsByQuote(E, curr, "") {
		if debug {
			fmt.Println(a.Name, i)
		}
		for _, b := range pairsByQuote(E, a.Base, a.Quote) {
			if c, err := E.Pair(b.Base + curr); err == nil {
				Sets = append(Sets, set{
					asset: curr,
					route: route{buy, buy, sell},
					a:     a,
					b:     b,
					c:     c,
				})
				if debug {
					fmt.Printf("buy ---- %-10s buy ---- %-10s sell --- %-10s\n", a.Name, b.Name, c.Name)
				}
			}
		}
	}
	if debug {
		fmt.Printf("--- total: %d ---\n", len(Sets))
	}
	return
}

func (s *bss) Sets(curr string) (Sets sets) {
	curr = strings.ToUpper(curr)

	for i, a := range pairsByQuote(E, curr, "") {
		if debug {
			fmt.Println(a.Name, i)
		}
		for _, b := range pairsByBase(E, a.Base, a.Quote) {
			if c, err := E.Pair(b.Quote + curr); err == nil {
				Sets = append(Sets, set{
					asset: curr,
					route: route{buy, sell, sell},
					a:     a,
					b:     b,
					c:     c,
				})
				if debug {
					fmt.Printf("buy ---- %-10s sell --- %-10s sell --- %-10s\n", a.Name, b.Name, c.Name)
				}
			}
		}
	}
	if debug {
		fmt.Printf("--- total: %d ---\n", len(Sets))
	}
	return
}

func (s *sbb) Sets(curr string) (Sets sets) {
	curr = strings.ToUpper(curr)

	for i, a := range pairsByBase(E, curr, "") {
		if debug {
			fmt.Println(a.Name, i)
		}
		for _, b := range pairsByQuote(E, a.Quote, a.Base) {
			if c, err := E.Pair(curr + b.Base); err == nil {
				Sets = append(Sets, set{
					asset: curr,
					route: route{sell, buy, buy},
					a:     a,
					b:     b,
					c:     c,
				})
				if debug {
					fmt.Printf("sell --- %-10s buy ---- %-10s buy ---- %-10s\n", a.Name, b.Name, c.Name)
				}
			}

		}
	}
	if debug {
		fmt.Printf("--- total: %d ---\n", len(Sets))
	}
	return
}

func (s *ssb) Sets(curr string) (Sets sets) {
	curr = strings.ToUpper(curr)

	for i, a := range pairsByBase(E, curr, "") {
		if debug {
			fmt.Println(a.Name, i)
		}
		for _, b := range pairsByBase(E, a.Quote, a.Base) {
			if c, err := E.Pair(curr + b.Quote); err == nil {
				Sets = append(Sets, set{
					asset: curr,
					route: route{sell, sell, buy},
					a:     a,
					b:     b,
					c:     c,
				})
				if debug {
					fmt.Printf("sell --- %-10s sell --- %-10s buy ---- %-10s\n", a.Name, b.Name, c.Name)
				}
			}
		}
	}
	if debug {
		fmt.Printf("--- total: %d ---\n", len(Sets))
	}
	return
}

func mapSets() {
	for _, currency := range MapAssets() {
		for _, fn := range arbs {
			for _, set := range fn.Sets(currency) {
				SetsMap[set.a] = append(SetsMap[set.a], set)
			}
		}
	}
}

func typeName(v interface{}) string {
	return fmt.Sprintf("%T", v)[6:]
}
