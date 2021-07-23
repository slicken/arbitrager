package main

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/slicken/arbitrager/currencie"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/orderbook"
)

type side byte

const (
	MAKER_FEE      = 0.00101
	buy       side = 0
	sell      side = 1
)

var Side = map[side]string{
	side(buy):  "buy",
	side(sell): "sell",
}

type Route [3]side

type Set struct {
	asset string
	route Route
	a     currencie.Pair
	b     currencie.Pair
	c     currencie.Pair
}

type Sets []Set

type Arbitrage interface {
	Sets(string) Sets
}

type bbs struct{ Route }
type bss struct{ Route }
type sbb struct{ Route }
type ssb struct{ Route }

var routes = []Arbitrage{
	&bbs{Route{buy, buy, sell}},
	&bss{Route{buy, sell, sell}},
	&sbb{Route{sell, buy, buy}},
	&ssb{Route{sell, sell, sell}},
}

var SetsMap = make(map[currencie.Pair]Sets)

func TopSetsMapList() []string {
	var points [3][]struct {
		Key   string
		Value int
	}

	abc := make([]map[string]int, 3)
	abc[0] = make(map[string]int)
	abc[1] = make(map[string]int)
	abc[2] = make(map[string]int)

	for n, m := range abc {
		for _, sets := range SetsMap {
			for _, set := range sets {
				m[set.a.Name]++
			}
		}
		for k, v := range m {
			points[n] = append(points[n], struct {
				Key   string
				Value int
			}{
				Key:   k,
				Value: v,
			})
		}
		sort.Slice(points[n], func(i, j int) bool {
			return points[n][i].Value > points[n][j].Value
		})

	}
	var ss []string
	for i, v := range points {
		for _, p := range v {
			ss = append(ss, p.Key)
			fmt.Printf("%-3d %-10s  =>  %-10d\n", i, p.Key, p.Value)
		}
	}
	return ss
}

func SetMapList() []string {
	var ss []string
	for _, sets := range SetsMap {
		for _, set := range sets {
			if !strings.Contains(fmt.Sprintln(ss), set.a.Name) {
				ss = append(ss, set.a.Name)
			}
			if !strings.Contains(fmt.Sprintln(ss), set.b.Name) {
				ss = append(ss, set.b.Name)
			}
			if !strings.Contains(fmt.Sprintln(ss), set.c.Name) {
				ss = append(ss, set.c.Name)
			}
		}
	}
	return ss
}

func (s Sets) List() []string {
	var ss []string
	for _, set := range s {
		if !strings.Contains(fmt.Sprintln(ss), set.a.Name) {
			ss = append(ss, set.a.Name)
		}
		if !strings.Contains(fmt.Sprintln(ss), set.b.Name) {
			ss = append(ss, set.b.Name)
		}
		if !strings.Contains(fmt.Sprintln(ss), set.c.Name) {
			ss = append(ss, set.c.Name)
		}
	}
	return ss
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

func (s *bbs) Sets(curr string) (sets Sets) {
	curr = strings.ToUpper(curr)

	for _, a := range pairsByQuote(E, curr, "") {
		for _, b := range pairsByQuote(E, a.Base, a.Quote) {
			if c, err := E.Pair(b.Base + curr); err == nil {
				sets = append(sets, Set{
					asset: curr,
					route: Route{buy, buy, sell},
					a:     a,
					b:     b,
					c:     c,
				})
			}
		}
	}
	return
}

func (s *bss) Sets(curr string) (sets Sets) {
	curr = strings.ToUpper(curr)

	for _, a := range pairsByQuote(E, curr, "") {
		for _, b := range pairsByBase(E, a.Base, a.Quote) {
			if c, err := E.Pair(b.Quote + curr); err == nil {
				sets = append(sets, Set{
					asset: curr,
					route: Route{buy, sell, sell},
					a:     a,
					b:     b,
					c:     c,
				})
			}
		}
	}
	return
}

func (s *sbb) Sets(curr string) (sets Sets) {
	curr = strings.ToUpper(curr)

	for _, a := range pairsByBase(E, curr, "") {
		for _, b := range pairsByQuote(E, a.Quote, a.Base) {
			if c, err := E.Pair(curr + b.Base); err == nil {
				sets = append(sets, Set{
					asset: curr,
					route: Route{sell, buy, buy},
					a:     a,
					b:     b,
					c:     c,
				})
			}
		}
	}
	return
}

func (s *ssb) Sets(curr string) (sets Sets) {
	curr = strings.ToUpper(curr)

	for _, a := range pairsByBase(E, curr, "") {
		for _, b := range pairsByBase(E, a.Quote, a.Base) {
			if c, err := E.Pair(curr + b.Quote); err == nil {
				sets = append(sets, Set{
					asset: curr,
					route: Route{sell, sell, buy},
					a:     a,
					b:     b,
					c:     c,
				})
			}
		}
	}
	return
}

func mapSets() {
	for _, currency := range assets {
		for _, fn := range routes {
			for _, set := range fn.Sets(currency) {
				SetsMap[set.a] = append(SetsMap[set.a], set)
			}
		}
	}
}

func containList(str string, list []string) bool {
	for _, v := range list {
		if strings.Contains(str, v) {
			return true
		}
	}
	return false
}

type OrderSet struct {
	profit float64
	amount float64
	next1  float64
	next2  float64
	next3  float64
	msg    string
	Set
}

// calcStepProfits looks for highest profits with a decreasing amount loop
func (s Set) calcStepProfits(amount float64) *OrderSet {
	var profits = make([]OrderSet, 0)

	for stepAmount := amount; stepAmount > 0; stepAmount -= (amount / float64(steps)) {
		if profit, a1, a2, a3, msg := s.calcDepthProfits(stepAmount); profit > 0 {
			profits = append(profits, OrderSet{profit: profit, amount: stepAmount, next1: a1, next2: a2, next3: a3, msg: msg})
		}
	}
	if len(profits) == 0 {
		return nil
	}

	sort.SliceStable(profits, func(i, j int) bool {
		return profits[i].profit > profits[j].profit
	})

	log.Printf(profits[0].msg, "==>")
	// log.Printf("%-6s %-12f --> %-12f (%5.2f%%) %8s %-12s %8s %-12s %8s %-12s\n", s.asset, profits[0].amount, profits[0].profit, perc, Side[s.route[0]], s.a.Name, Side[s.route[1]], s.b.Name, Side[s.route[2]], s.c.Name)
	profits[0].Set = s
	return &profits[0]
}

// calcDepthProfits returns real ask/bid prices depending on amount depth
func (s Set) calcDepthProfits(amount float64) (float64, float64, float64, float64, string) {
	var pair = [3]string{s.a.Name, s.b.Name, s.c.Name}
	var price = [3]float64{0, 0, 0}
	var next [3]float64

	nextAmount := amount
	for i, _action := range s.route {
		book, _ := orderbook.GetBook(pair[i])

		//		   buyFee=BASE_QUOTE=sellFee				  next=is amount orders will use
		// 		action	pair			price			[]next			nextAmount		correct			comment
		// loop 				300
		// 	0	0BUY	PSGUSDT		/	16.543000	=	18,134558424	18,116423865	18,134558424	buy  amount
		// 	1	1SELL	PSGBTC      *	0.000518	=	18,116423865	0,009374923		18,116423865	sell amount
		// 	2	1SELL	BTCUSDT     *	32498.63000	=	304,672162114	304,367489952	0,009374923		sell amount
		//						300
		// 	0	0BUY	BTCUSDT		/	32498.63000	=	0,009231158		0,009221927		0,009231158		buy  amount
		// 	1	0BUY	PSGBTC      /	0.000518	=	17,802948266	17,785145317	17,802948266	buy  amount
		// 	2	1SELL	PSGUSDT     *	16.543000	=	17,785145317	293,92543932	17,785145317	sell amount

		switch _action {
		// buy
		case 0:
			asks := book.Asks.Get()
			for _, depth := range asks {
				if depth.Total >= (nextAmount / depth.Price) {
					price[i] = depth.Price
					nextAmount /= depth.Price
					next[i] = nextAmount
					nextAmount -= (nextAmount * MAKER_FEE)
					break
				}
			}
		// sell
		case 1:
			bids := book.Bids.Get()
			for _, depth := range bids {
				if depth.Total >= nextAmount {
					price[i] = depth.Price
					nextAmount *= depth.Price
					next[i] = nextAmount / price[i]
					nextAmount -= (nextAmount * MAKER_FEE)
					break
				}
			}
		}
		if price[i] == 0 {
			return 0, 0, 0, 0, ""
		}
	}

	profit := nextAmount - amount
	perc := ((amount+profit)/amount)*100 - 100
	if 0 >= profit {
		return 0, 0, 0, 0, ""
	}
	msg := fmt.Sprintf("%-6s %-12f %%s %-12f (%5.2f%%%%) %8s %-12s %-12f %8s %-12s %-12f %8s %-12s %-12f\n",
		s.asset, amount, profit, perc, Side[s.route[0]], pair[0], price[0], Side[s.route[1]], pair[1], price[1], Side[s.route[2]], pair[2], price[2])
	if verbose {
		log.Printf(msg, "   ")
	}
	if target > perc {
		return 0, 0, 0, 0, ""
	}

	return profit, next[0], next[1], next[2], msg
}
