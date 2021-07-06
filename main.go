package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/slicken/arbitrager/config"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/orderbook"
)

var (
	E        exchanges.I
	shutdown = make(chan bool)

	assets []string
	except []string
	debug  = false
	all    = false
)

func appInfo() {
	_, app := filepath.Split(os.Args[0])
	fmt.Println(`Usage: ./` + app + ` [-a <quote>|--all] [-e <curr>] [--debug]
       ./` + app + ` -a BTC,ETH                  only thease assets
       ./` + app + ` -e DOT,USDC                 except thease
       ./` + app + ` --all                       all assets with balance
       ./` + app + ` --debug
                                      -- slk prod 2021 --`)
	os.Exit(0)
}

// Split makes slice of string
func Split(s string) []string {
	var slice []string
	for _, v := range strings.Split(s, ",") {
		slice = append(slice, v)
	}
	return slice
}

func main() {

	// APP ARGUMENTS
	if 2 > len(os.Args) {
		appInfo()
	} else {
		for i, arg := range os.Args[1:] {
			// fmt.Printf("i=%d  arg=%s\n", i, arg)
			switch arg {
			case "-a":
				if i+3 > len(os.Args) {
					appInfo()
				}
				assets = Split(os.Args[i+2])
				log.Println("arbitraging", assets)

			case "-e":
				if i+3 > len(os.Args) {
					appInfo()
				}
				except = Split(os.Args[i+2])
				log.Println("except", except)

			case "--all":
				all = true
				log.Println("all currencie with balance")

			case "--debug":
				debug = true
				log.Println("enabled debugging")

			default:
			}

		}
	}

	HandleInterrupt()

	// LOAD CONFIG FILE
	if err := config.ReadConfig(); err != nil {
		log.Fatalf("could not load config file: %v\n", err)
	}
	log.Println("reading config...")

	// LOAD EXCHANGE
	if err := LoadExchange("binance"); err != nil {
		log.Fatalf("could not load exchange: %v\n", err)
	}
	log.Println("connected to", E.GetName())

	// time.Sleep(time.Second)
	// tmp ---->
	// numPairs := 0
	// for _ = range E.AllPairs() {
	// 	numPairs++
	// }
	// if numPairs == 0 {
	// 	log.Fatal("binance didnt load succesfully, numPairs =", numPairs)
	// }
	// // end ----<

	// ARBITRAGE
	mapSets()

	if all {
		// for asset := range balance.Balances {
		for asset := range MapAssets() {
			assets = append(assets, asset)
		}
	}
	for _, e := range except {
		for i, t := range assets {
			if e == t {
				assets = append(assets[:i], assets[i+1:]...)
			}
		}
	}

	// TODO: create balance/amount maitainer so we know maximum amount/asset we can look arbitrage opportunities of
	// maby a balance.Update after arbitrage is enuff

	// create pairs to subcribe to
	var pairs []string
	for _, a := range assets {
		for _, fn := range arbs {
			pairs = append(pairs, fn.Sets(a).List()...)
		}
	}

	// handle ws streams
	var handlarC = make(chan []byte, len(pairs))

	// handle orderbook data
	go func() {
		for {
			select {

			// orderbook
			case b := <-handlarC:
				var resp WsDepthEvent
				if err := json.Unmarshal(b, &resp); err != nil {
					log.Println(err)
					continue
				}

				book, _ := orderbook.GetBook(resp.Symbol)

				for _, v := range resp.Asks {
					p, _ := strconv.ParseFloat(v[0].(string), 64)
					a, _ := strconv.ParseFloat(v[1].(string), 64)
					book.Asks.Add(p, a)
				}
				for _, v := range resp.Bids {
					p, _ := strconv.ParseFloat(v[0].(string), 64)
					a, _ := strconv.ParseFloat(v[1].(string), 64)
					book.Bids.Add(p, a)
				}

				// look for arbitrage on all possible routes
				pair, err := E.Pair(resp.Symbol)
				if err != nil {
					continue
				}
				sets := SetsMap[pair]
				for _, set := range sets {
					for _, asset := range assets {
						if asset != set.asset {
							continue
						}
						if set.calcProfit(0) {
							// make orders
						}
					}
				}

				// if debug {
				// 	if len(book.Asks) > 0 {
				// 		v := book.Asks.Get()[0]
				// 		fmt.Printf("%-16s %-16.4f %-16.4f %-16.4f\n", book.Name, v.Price, v.Amount, v.Total)

				// 		// cls()
				// 		// for _, v := range book.Asks.Get() {
				// 		// 	fmt.Printf("%-16s %-16.4f %-16.4f %-16.4f\n", book.Name, v.Price, v.Amount, v.Total)
				// 		// }
				// 	}
				// }

			default:
			}
		}
	}()

	// subscribe orderbook data
	for _, pair := range pairs {
		subscribePair(pair, handlarC)
	}

	<-shutdown
}

func subscribePair(name string, handler chan<- []byte) {
	u := url.URL{
		Scheme: "wss",
		Host:   "stream.binance.com:9443",
		Path:   fmt.Sprintf("/ws/%s@depth", strings.ToLower(name)),
		// Path: fmt.Sprintf("/ws/%s@depth%s@100ms", strings.ToLower(name), "10"),
	}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("dial:", err)
		return
	}
	log.Println("subscribed to", name)

	go func(name string) {
		defer c.Close()

		for {
			if _, message, err := c.ReadMessage(); err != nil {
				log.Println("ws error:", err)
			} else {
				handler <- message
			}
		}
	}(name)

}

// WsDepthEvent - ws orderbook data
type WsDepthEvent struct {
	Event         string          `json:"e"`
	Time          int64           `json:"E"`
	Symbol        string          `json:"s"`
	LastUpdateID  int64           `json:"u"`
	FirstUpdateID int64           `json:"U"`
	Bids          [][]interface{} `json:"b"`
	Asks          [][]interface{} `json:"a"`
}

// DepthResponse for fast ws orderbook data
type DepthResponse struct {
	LastUpdateID int64           `json:"lastUpdateId"`
	Bids         [][]interface{} `json:"bids"`
	Asks         [][]interface{} `json:"asks"`
}

func cls() {
	cls := exec.Command("clear")
	cls.Stdout = os.Stdout
	cls.Run()
}

// MapQuotes ..
func MapQuotes() map[string]string {
	var m = make(map[string]string)
	for _, v := range E.AllPairs() {
		if _, ok := m[v.Quote]; ok {
			continue
		}
		m[v.Quote] = v.Quote
	}
	return m
}

// MapQuotes ..
func MapBases() map[string]string {
	var m = make(map[string]string)
	for _, v := range E.AllPairs() {
		if _, ok := m[v.Base]; ok {
			continue
		}
		m[v.Base] = v.Base
	}
	return m
}

func MapAssets() map[string]string {
	var m = make(map[string]string)
	for _, v := range E.AllPairs() {
		if _, ok := m[v.Quote]; ok {
			continue
		}
		m[v.Quote] = v.Quote
	}
	for _, v := range E.AllPairs() {
		if _, ok := m[v.Base]; ok {
			continue
		}
		m[v.Base] = v.Base
	}
	return m
}

// HandleInterrupt securly exits bot
func HandleInterrupt() {
	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	go func() {

		select {
		case <-interrupt:
			break
		}

		log.Println("shutting down...")
		close(shutdown)
	}()
}
