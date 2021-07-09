package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/slicken/arbitrager/balance"
	"github.com/slicken/arbitrager/config"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/orderbook"
)

var (
	E        exchanges.I
	shutdown = make(chan bool)
	// app args
	assets  []string
	except  []string
	all     = false
	target  = 0.2
	minimum = 100.
	cpu     = 0
	debug   = false

	// helper
	tickers    map[string]float64
	_, appName = filepath.Split(os.Args[0])
)

func appInfo() {
	fmt.Println(`Usage: ./` + appName + ` [-a <assets>|--all] [-e <assets>] [-t <percent>] [-m <USD>]
             [--CPU <cores>] [--debug]
Arguments
  -a, --asset   BTC,ETH,BNB              only thease assets.  TIP use quote assets
      --all                              all assets with balance
  -e, --except  DOT,USDC                 except thease assets
  -t, --target  0.5                      target percent            (default is 0.2)
  -m, --minimum 500                      mimimum balance (in USD)  (default is 100)
      --CPU 2                            limit cpu cores           (default is max)
      --debug
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
			case "-a", "--asset":
				if i+3 > len(os.Args) {
					appInfo()
				}
				assets = Split(os.Args[i+2])
				log.Println("arbitraging", assets)

			case "--all":
				all = true
				log.Println("all assets with balance")

			case "-e", "--except":
				if i+3 > len(os.Args) {
					appInfo()
				}
				except = Split(os.Args[i+2])
				log.Println("except", except)

			case "-t", "--target":
				if i+3 > len(os.Args) {
					appInfo()
				}
				tmp, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo()
				}
				target = tmp
				log.Println("target percent", target)

			case "-m", "--minimum":
				if i+3 > len(os.Args) {
					appInfo()
				}
				tmp, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo()
				}
				minimum = tmp
				log.Println("minimum balance (in USD)", minimum)

			case "--CPU":
				if i+3 > len(os.Args) {
					appInfo()
				}
				tmp, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo()
				}
				cpu = tmp
				log.Println("cpu cores limited to", cpu)

			case "--debug":
				debug = true
				log.Println("debug enabled")
				log.Println("10,000 USDT test account")

			default:
			}
		}
	}

	HandleInterrupt()

	// LOG TO FILE
	LogToFile(appName)

	// LIMIT CPU CORES
	if cpu != 0 {
		runtime.GOMAXPROCS(cpu)
	}
	// LOAD CONFIG FILE
	if err := config.ReadConfig(); err != nil {
		log.Fatalln("could not load config file:", err)
	}
	log.Println("reading config...")

	// LOAD EXCHANGE
	if err := LoadExchange("binance"); err != nil {
		log.Fatalln("could not load exchange:", err)
	}
	log.Println("connected to", E.GetName())

	// PREPARE DATA

	// update all tickers every hour
	go func() {
		ticker := time.NewTicker(time.Hour)

		for ; true; <-ticker.C {
			var err error

			tickers, err = E.GetAllTickers()
			if err != nil {
				log.Println("failed to get all tickers:", err)
			}
		}
	}()

	if all {
		if debug {
			for asset := range MapAssets() {
				assets = append(assets, asset)
			}
		} else {
			for asset := range balance.Balances {
				//check if balance is worth more than mimimum
				free := balance.Balances[asset].Free
				_pair, err := E.Pair(asset + "USDT")
				if err == nil {
					free *= tickers[_pair.Name]
				}
				if minimum > free {
					continue
				}
				assets = append(assets, asset)
			}
		}
	}
	for _, e := range except {
		for i, t := range assets {
			if e == t {
				assets = append(assets[:i], assets[i+1:]...)
			}
		}
	}

	mapSets()

	// create pairs to subcribe to
	var pairs []string
	for _, a := range assets {
		for _, fn := range arbs {
			pairs = append(pairs, fn.Sets(a).List()...)
		}
	}

	//
	// TODO: limmit pairs to exchange maximum
	//       what is Binance, Kucoin, FTX maximum ws
	//
	pairs = pairs[:1024]
	log.Printf("%s total: %d\n", pairs, len(pairs))

	// handle ws streams
	var handlarC = make(chan []byte, len(pairs))

	// handle orderbook data
	go func() {
		for {
			select {
			case <-shutdown:
				return
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

						if debug {
							free := 10000.
							set.calcProfit(free)
							continue
						}
						//check if balance is worth more than mimimum
						free := balance.Balances[asset].Free
						_pair, err := E.Pair(asset + "USDT")
						if err == nil {
							free *= tickers[_pair.Name]
						}
						if minimum > free {
							continue
						}
						// check is we can profit
						if tradesize := set.calcProfit(balance.Balances[asset].Free); tradesize >= minimum {
							// calc tradesize1
							// E.SendMarket(set.a.Name, actions[set.route[0]], tradesize1)
							// // calc tradesize2
							// E.SendMarket(set.b.Name, actions[set.route[1]], tradesize2)
							// // calc tradesize3
							// E.SendMarket(set.c.Name, actions[set.route[2]], tradesize3)
						}
					}
				}

			default:
			}
		}
	}()

	// subscribe streams

	// subscribeAllPairs(handlerC)

	for _, pair := range pairs {
		select {
		case <-shutdown:
			return
		default:
		}
		subscribePair(pair, handlarC)
	}

	<-shutdown
}

func subscribePair(name string, handler chan<- []byte) {
	u := url.URL{
		Scheme: "wss",
		Host:   "stream.binance.com:9443",
		Path:   fmt.Sprintf("/ws/%s@depth", strings.ToLower(name)),
	}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("dial:", err)
		return
	}
	log.Println("subscribed to", name)

	go func() {
		defer c.Close()

		keepAlive(c, time.Minute)
		for {
			if _, message, err := c.ReadMessage(); err != nil {
				log.Println("ws error:", err)
				break
			}
			handler <- message
		}
	}()
}

var wsCombinedPath = "/stream?streams="

func subscribeAllPairs(handler chan<- []byte) {
	path := wsCombinedPath
	for _, v := range pairs {
		path += fmt.Sprintf("%s@depth%s", strings.ToLower(v), "" /* "@20@100ms" */) + "/"
	}
	path = path[:len(path)-1]
	
	u := url.URL{
		Scheme: "wss",
		Host:   "stream.binance.com:9443",
		Path:	path,            // Path:   "/ws/",                           // do we need path?
	}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("dial:", err)
		return
	}
/*
	var pairList []string
	for _, pair := range pairs {
		pairList = append(pairList, strings.ToLower(name) + "@depth")
		// pairList = append(pairList, fmt.Sprintf("%s@depth@%d@100ms", strings.ToLower(name), 20), // valid levels are 5, 10, or 20
	}
	
	// json payload
	var payload struct {
		method string,
		params []string,
		id uint64
	}{
		"method": "SUBSCRIBE",
		"params": pairList,
		"id": 731501
	}

	// marshal payload
	// err := c.JSON.WriteMessage(websocket.TextMessage, payload)
	
	err := websocket.JSON.Send(c, payload)
	if err != nil {
		log.Println("write message:", err)
		c.Close()
		return
	}
*/	
	log.Println("subscribed to", pairs)

	go func() {
		defer c.Close()

		keepAlive(c, time.Minute)
		for {
			if _, message, err := c.ReadMessage(); err != nil {
				log.Println("ws error:", err)
				break
			}
			handler <- message
		}
	}()
}

func keepAlive(c *websocket.Conn, timeout time.Duration) {
	ticker := time.NewTicker(timeout)

	lastResponse := time.Now()
	c.SetPongHandler(func(msg string) error {
		lastResponse = time.Now()
		return nil
	})

	go func() {
		defer ticker.Stop()
		for {
			deadline := time.Now().Add(10 * time.Second)
			err := c.WriteControl(websocket.PingMessage, []byte{}, deadline)
			if err != nil {
				c.Close()
				return
			}
			<-ticker.C
			if time.Since(lastResponse) > timeout {
				c.Close()
				return
			}
		}
	}()
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

// LogToFile ...
func LogToFile(tag string) {
	if tag != "" {
		tag = tag + "_"
	}
	logName := tag + time.Now().Format("20060102") + ".log"
	logFile, err := os.Create(logName)
	if err != nil {
		log.Fatalf("could not create %q: %v", logFile.Name(), err)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	log.Printf("successfully created logfile %q.\n", logFile.Name())
}
