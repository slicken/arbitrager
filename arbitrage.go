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
	MAKER_FEE      = 0.001
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
	pair  [3]currencie.Pair
	// --- delete ---
	a currencie.Pair
	b currencie.Pair
	c currencie.Pair
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

// func TopSetsMapList() []string {
// 	var points [3][]struct {
// 		Key   string
// 		Value int
// 	}

// 	abc := make([]map[string]int, 3)
// 	abc[0] = make(map[string]int)
// 	abc[1] = make(map[string]int)
// 	abc[2] = make(map[string]int)

// 	for n, m := range abc {
// 		for _, sets := range SetsMap {
// 			for _, set := range sets {
// 				m[set.a.Name]++
// 			}
// 		}
// 		for k, v := range m {
// 			points[n] = append(points[n], struct {
// 				Key   string
// 				Value int
// 			}{
// 				Key:   k,
// 				Value: v,
// 			})
// 		}
// 		sort.Slice(points[n], func(i, j int) bool {
// 			return points[n][i].Value > points[n][j].Value
// 		})

// 	}
// 	var ss []string
// 	for i, v := range points {
// 		for _, p := range v {
// 			ss = append(ss, p.Key)
// 			fmt.Printf("%-3d %-10s  =>  %-10d\n", i, p.Key, p.Value)
// 		}
// 	}
// 	return ss
// }

func SetMapList() []string {
	var ss []string
	for _, sets := range SetsMap {
		for _, set := range sets {
			for _, p := range set.pair {
				if !strings.Contains(fmt.Sprintln(ss), p.Name) {
					ss = append(ss, p.Name)
				}
			}
		}
	}
	return ss
}

func (s Sets) List() []string {
	var ss []string
	for _, set := range s {
		for _, p := range set.pair {
			if !strings.Contains(fmt.Sprintln(ss), p.Name) {
				ss = append(ss, p.Name)
			}
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
					pair:  [3]currencie.Pair{a, b, c},
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
					pair:  [3]currencie.Pair{a, b, c},
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
					pair:  [3]currencie.Pair{a, b, c},
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
					pair:  [3]currencie.Pair{a, b, c},
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
				for _, p := range set.pair {
					SetsMap[p] = append(SetsMap[p], set)
				}
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
	initial float64
	profit  float64
	perc    float64
	amount  [3]float64
	price   [3]float64
	msg     string
	Set
}

// calcStepProfits looks for highest profits with a decreasing amount loop
func (s Set) calcStepProfits(amount float64) *OrderSet {
	var os = make([]*OrderSet, 0)

	for size := amount; size > 0; size -= (amount / float64(steps)) {
		if o := s.calcDepthProfits(size); o != nil {
			os = append(os, o)
		}
	}
	if len(os) == 0 {
		return nil
	}

	sort.SliceStable(os, func(i, j int) bool {
		return os[i].profit > os[j].profit
	})

	log.Printf(os[0].msg, "==>")
	os[0].Set = s
	return os[0]
}

// calcDepthProfitsOld returns real ask/bid prices depending on amount depth
func (s Set) calcDepthProfitsOld(amount float64) *OrderSet {
	o := &OrderSet{
		initial: amount,
		amount:  [3]float64{0, 0, 0},
		price:   [3]float64{0, 0, 0},
	}

	next := amount
	for i, _action := range s.route {
		book, _ := orderbook.GetBook(s.pair[i].Name)

		//		   buyFee	Base
		//		   sellFee	Quote						  amount=is amount orders will use
		// 		action	pair			price			[]amount		nextAmount		correct			comment			total
		// loop 				300
		// 	0	0BUY	PSGUSDT		/	16.543000	=	18,134558424	18,116423865	18,134558424	buy  amount		300			initial
		// 	1	1SELL	PSGBTC      *	0.000518	=	18,116423865	0,009374923		18,116423865	sell amount		0,00937		nextAmount
		// 	2	1SELL	BTCUSDT     *	32498.63000	=	304,672162114	304,367489952	0,009374923		sell amount		304,367		nextAmount
		//						300
		// 	0	0BUY	BTCUSDT		/	32498.63000	=	0,009231158		0,009221927		0,009231158		buy  amount		0,009231158	amount
		// 	1	0BUY	PSGBTC      /	0.000518	=	17,802948266	17,785145317	17,802948266	buy  amount		0,009231158	amount -1
		// 	2	1SELL	PSGUSDT     *	16.543000	=	17,785145317	293,92543932	17,785145317	sell amount		293,9254	nextAmount
		//						100
		// 	0	0BUY	BNBUSDT		/	302.870000	=	0,330174662		0,329844488		0,330174662		buy  amount		100			initial
		// 	1	0BUY	C98BNB      /	0.005900	=	55,905845424	55,849939578	55,905845424	buy  amount		0,330174662	amount -1
		// 	2	1SELL	C98USDT     *	1.830000	=	55,849939578	102,205389428	55,849939578	sell amount		102,2053894 nextAmount

		switch _action {
		// buy
		case 0:
			asks := book.Asks.Get()
			for _, depth := range asks {
				if depth.Total >= (next / depth.Price) {
					o.price[i] = depth.Price
					next /= depth.Price
					o.amount[i] = next
					next -= (next * MAKER_FEE)
					break
				}
			}
		// sell
		case 1:
			bids := book.Bids.Get()
			for _, depth := range bids {
				if depth.Total >= next {
					o.price[i] = depth.Price
					next *= depth.Price
					o.amount[i] = next / o.price[i]
					next -= (next * MAKER_FEE)
					break
				}
			}
		}
		if o.price[i] == 0 {
			return nil
		}
	}

	o.profit = next - o.initial
	o.perc = ((o.initial+o.profit)/o.initial)*100 - 100
	if 0.1 > o.perc {
		return nil
	}
	o.msg = fmt.Sprintf("%-6s %-12f %%s %-12f (%5.2f%%%%) %8s %-12s %-12f %8s %-12s %-12f %8s %-12s %-12f\n",
		s.asset, o.initial, o.profit, o.perc, Side[s.route[0]], s.pair[0].Name, o.price[0], Side[s.route[1]], s.pair[1].Name, o.price[1], Side[s.route[2]], s.pair[2].Name, o.price[2])
	if verbose {
		log.Printf(o.msg, "   ")
	}
	if target > o.perc {
		return nil
	}

	return o
}

// calcDepthProfits calculates triangular arbitrage profits
// using real ask/bid prices from orderbook price depth
func (s Set) calcDepthProfits(amount float64) *OrderSet {
	o := &OrderSet{
		initial: amount,
		amount:  [3]float64{0, 0, 0},
		price:   [3]float64{0, 0, 0},
	}

	next := amount
	for i, _action := range s.route {
		book, _ := orderbook.GetBook(s.pair[i].Name)

		switch _action {
		case 0: // buy
			dp := book.Asks.GetDepthPrice(next - (MAKER_FEE * next))
			if dp < 0 {
				return nil
			}
			o.price[i] = dp
			next /= dp
			o.amount[i] = next
		case 1: // sell
			dp := book.Bids.GetDepthPrice(next - (MAKER_FEE * next))
			if dp < 0 {
				return nil
			}
			o.price[i] = dp
			next *= dp
			o.amount[i] = next / o.price[i]
		}
	}

	o.profit = next - o.initial
	o.perc = ((o.initial+o.profit)/o.initial)*100 - 100
	if 0.1 > o.perc {
		return nil
	}
	o.msg = fmt.Sprintf("%-6s %-12f %%s %-12f (%5.2f%%%%) %8s %-12s %-12f %8s %-12s %-12f %8s %-12s %-12f\n",
		s.asset, o.initial, o.profit, o.perc, Side[s.route[0]], s.pair[0].Name, o.price[0], Side[s.route[1]], s.pair[1].Name, o.price[1], Side[s.route[2]], s.pair[2].Name, o.price[2])
	if verbose {
		log.Printf(o.msg, "   ")
	}
	if target > o.perc {
		return nil
	}

	return o
}
