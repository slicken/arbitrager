package main

import (
	"log"
	"sort"
	"strings"

	"github.com/slicken/arbitrager/currencie"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/orderbook"
)

type action byte

const (
	MAKER_FEE        = 0.001
	buy       action = 0
	sell      action = 1
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

func (s set) calcProfit(amount float64) float64 {
	var pair = [3]string{s.a.Name, s.b.Name, s.c.Name}
	var price [3]float64

	for i, _action := range s.route {
		book, _ := orderbook.GetBook(pair[i])

		// get the right prices
		var depth []orderbook.Item
		switch _action {
		case 0:
			depth = book.Asks.Get()
		case 1:
			depth = book.Bids.Get()
		}

		if len(depth) == 0 {
			return 0
		}

		price[i] = depth[0].Price
	}

	var profit float64
	switch s.route {
	case route{0, 0, 1}:
		profit = amount/price[0]/price[1]*price[2] - (amount + (amount * 0.003))
	case route{0, 1, 1}:
		profit = amount/price[0]*price[1]*price[2] - (amount + (amount * 0.003))
	case route{1, 0, 0}:
		profit = amount*price[0]/price[1]/price[2] - (amount + (amount * 0.003))
	case route{1, 1, 0}:
		profit = amount*price[0]*price[1]/price[2] - (amount + (amount * 0.003))
	}

	newAmount := amount + profit
	perc := (newAmount/amount)*100 - 100

	if perc >= target || verbose {
		log.Printf("%s %-12f (%6.2f%%) %12s %-10s %-12f %12s %-10s %-12f %12s %-10s %-12f\n", s.asset, profit, perc, actions[s.route[0]], pair[0], price[0], actions[s.route[1]], pair[1], price[1], actions[s.route[2]], pair[2], price[2])
		return 1
	}

	return 0
}

type profitAmount struct {
	profit float64
	amount float64
}

func (s set) calcMaxProfit(amount float64) float64 {
	var pair = [3]string{s.a.Name, s.b.Name, s.c.Name}
	var price [3]float64

	var profits = make([]profitAmount, 0)

	// stepAmount := amount
	// fmt.Printf("%s\tentering loop. %v\n", s.asset, pair)
	for stepAmount := amount; stepAmount > 0; stepAmount -= (amount / float64(steps)) {
		// for {

		// next:

		// 	if 0 >= stepAmount {
		// 		// fmt.Printf("%s\tbreak loop 0 > %f\n", s.asset, stepAmount)
		// 		break
		// 	}

		// fmt.Println("-------------------------------------------------------")
		// fmt.Printf("%s\tstepAmount=%f\n", s.asset, stepAmount)

		tempAmount := stepAmount
		for i, _action := range s.route {
			book, _ := orderbook.GetBook(pair[i])
			// fmt.Printf("%s\ti=%d\t%s\ttempAmount=%f\n", s.asset, i, pair[i], tempAmount)

			switch _action {
			case 0:
				for _, depth := range book.Asks.Get() {
					// fmt.Printf("%s\t%s\tdepth.Total=%f\tdepth.Price=%f\tdepth.Amount=%f\ttemp/price=%f\n", s.asset, actions[_action], depth.Total, depth.Price, depth.Amount, (tempAmount / depth.Price))
					if depth.Total >= (tempAmount / depth.Price) {
						tempAmount /= depth.Price
						price[i] = depth.Price
						// fmt.Printf("%s\tgot depth %s. tempAmount=%f\n", s.asset, actions[_action], tempAmount)
						break
					}
				}
			case 1:
				for _, depth := range book.Bids.Get() {
					// fmt.Printf("%s\t%s\tdepth.Total=%f\tdepth.Price=%f\tdepth.Amount=%f\ttemp*price=%f\n", s.asset, actions[_action], depth.Total, depth.Price, depth.Amount, (tempAmount * depth.Price))
					if depth.Total >= tempAmount { //(tempAmount * depth.Price) {
						tempAmount *= depth.Price
						price[i] = depth.Price
						// fmt.Printf("%s\tgot depth %s. tempAmount=%f\n", s.asset, actions[_action], tempAmount)
						break
					}
				}

			}
			// next step if no depth price
			if price[i] == 0 {
				// stepAmount -= (amount / float64(steps))
				// fmt.Printf("%s\tno depth. next stepAmount=%.f\n", s.asset, stepAmount)
				break
				// goto next
			}

		}

		var profit float64
		switch s.route {
		case route{0, 0, 1}:
			profit = stepAmount/price[0]/price[1]*price[2] - (stepAmount + (stepAmount * 0.003))
		case route{0, 1, 1}:
			profit = stepAmount/price[0]*price[1]*price[2] - (stepAmount + (stepAmount * 0.003))
		case route{1, 0, 0}:
			profit = stepAmount*price[0]/price[1]/price[2] - (stepAmount + (stepAmount * 0.003))
		case route{1, 1, 0}:
			profit = stepAmount*price[0]*price[1]/price[2] - (stepAmount + (stepAmount * 0.003))
		}

		// profit -= (profit * MAKER_FEE * 3)
		profits = append(profits, profitAmount{profit: profit, amount: stepAmount})

		// stepAmount -= (amount / float64(steps))
		// fmt.Printf("%s\tsaving profit=%.f\t. next stepAmount=%.f\n", s.asset, profit, stepAmount)
	}

	if len(profits) == 0 || price[0] == 0 || price[1] == 0 || price[2] == 0 {
		// fmt.Printf("%s\tno profits exiting\n", s.asset)
		return 0
	}

	// sort biggest profit first
	sort.SliceStable(profits, func(i, j int) bool {
		return profits[i].profit > profits[j].profit
	})

	// fmt.Printf("sorted profit, biggest%.f\t, smallest=%.f\n", profits[0].profit, profits[len(profits)-1].profit)

	profit := profits[0].profit
	amount = profits[0].amount
	newAmount := amount + profit
	perc := (newAmount/amount)*100 - 100

	if perc >= target || verbose {
		log.Printf("%s %-12f (%6.2f%%) %12s %-10s %-12f %12s %-10s %-12f %12s %-10s %-12f\n", s.asset, profit, perc, actions[s.route[0]], pair[0], price[0], actions[s.route[1]], pair[1], price[1], actions[s.route[2]], pair[2], price[2])
		return amount
	}
	// fmt.Println("=======================================================")

	return 0

}

func (s *bbs) Sets(curr string) (Sets sets) {
	curr = strings.ToUpper(curr)

	for _, a := range pairsByQuote(E, curr, "") {
		for _, b := range pairsByQuote(E, a.Base, a.Quote) {
			if c, err := E.Pair(b.Base + curr); err == nil {
				Sets = append(Sets, set{
					asset: curr,
					route: route{buy, buy, sell},
					a:     a,
					b:     b,
					c:     c,
				})
				if verbose {
					log.Printf("buy ---- %-10s buy ---- %-10s sell --- %-10s\n", a.Name, b.Name, c.Name)
				}
			}
		}
	}
	if verbose && len(Sets) > 0 {
		log.Printf("--- total: %d ---\n", len(Sets))
	}
	return
}

func (s *bss) Sets(curr string) (Sets sets) {
	curr = strings.ToUpper(curr)

	for _, a := range pairsByQuote(E, curr, "") {
		for _, b := range pairsByBase(E, a.Base, a.Quote) {
			if c, err := E.Pair(b.Quote + curr); err == nil {
				Sets = append(Sets, set{
					asset: curr,
					route: route{buy, sell, sell},
					a:     a,
					b:     b,
					c:     c,
				})
				if verbose {
					log.Printf("buy ---- %-10s sell --- %-10s sell --- %-10s\n", a.Name, b.Name, c.Name)
				}
			}
		}
	}
	if verbose && len(Sets) > 0 {
		log.Printf("--- total: %d ---\n", len(Sets))
	}
	return
}

func (s *sbb) Sets(curr string) (Sets sets) {
	curr = strings.ToUpper(curr)

	for _, a := range pairsByBase(E, curr, "") {
		for _, b := range pairsByQuote(E, a.Quote, a.Base) {
			if c, err := E.Pair(curr + b.Base); err == nil {
				Sets = append(Sets, set{
					asset: curr,
					route: route{sell, buy, buy},
					a:     a,
					b:     b,
					c:     c,
				})
				if verbose {
					log.Printf("sell --- %-10s buy ---- %-10s buy ---- %-10s\n", a.Name, b.Name, c.Name)
				}
			}

		}
	}
	if verbose && len(Sets) > 0 {
		log.Printf("--- total: %d ---\n", len(Sets))
	}
	return
}

func (s *ssb) Sets(curr string) (Sets sets) {
	curr = strings.ToUpper(curr)

	for _, a := range pairsByBase(E, curr, "") {
		for _, b := range pairsByBase(E, a.Quote, a.Base) {
			if c, err := E.Pair(curr + b.Quote); err == nil {
				Sets = append(Sets, set{
					asset: curr,
					route: route{sell, sell, buy},
					a:     a,
					b:     b,
					c:     c,
				})
				if verbose {
					log.Printf("sell --- %-10s sell --- %-10s buy ---- %-10s\n", a.Name, b.Name, c.Name)
				}
			}
		}
	}
	if verbose && len(Sets) > 0 {
		log.Printf("--- total: %d ---\n", len(Sets))
	}
	return
}

func mapSets() {
	if verbose {
		log.Println("mapping out trade routs for", assets)
	}
	for _, currency := range assets {
		for _, fn := range arbs {
			for _, set := range fn.Sets(currency) {
				SetsMap[set.a] = append(SetsMap[set.a], set)
			}
		}
	}
}
