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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/slicken/arbitrager/balance"
	"github.com/slicken/arbitrager/config"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/orderbook"
)

var (
	// args
	assets  []string
	except  []string
	all     = false
	target  = 0.5
	steps   = 1
	minimum = 200.
	level   = ""
	cpu     = 0
	verbose = false

	// -------
	E          exchanges.I
	tickers    map[string]float64
	shutdown   = make(chan bool)
	_, appName = filepath.Split(os.Args[0])

	lock = make(chan bool, 1)
)

func appInfo() {
	fmt.Println(`Usage: ./` + appName + ` [-a <assets>|--all] [-e <assets>] [-t <percent>] [-n <uint>] [-m <USD>]
             [--100] [--CPU <cores>] [--verbose]
Arguments        Examples
  -a, --asset    BTC,ETH,BNB              only thease assets. TIP use quote assets
      --all                               all assets with balance
  -e, --except   DOT,USDC                 except thease assets
  -t, --target   1.0                      target percent             (default 0.5)
  -n, --decrease 4                        decrease balance N times   (default 1  )
  -m, --minimum  500                      mimimum balance (in USD)   (default 200)
      --100                               fetch data every 100ms     (default 1s )
      --CPU      2                        limit cpu cores            (default max)
      --verbose
  -h  --help
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

			case "-n", "--decrease":
				if i+3 > len(os.Args) {
					appInfo()
				}
				tmp, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo()
				}
				steps = tmp
				log.Printf("decrease balance %d times\n", steps)

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

			case "--100":
				level = "20@100ms"
				log.Println("fetching data every 100ms")

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

			case "-h", "--help", "help":
				appInfo()

			case "--verbose":
				verbose = true
				log.Println("verbose enabled")
				log.Println("10,000 USDT test account")

			default:
			}
		}
	}
	if !all && len(assets) == 0 {
		appInfo()
	}

	// HANDLE INTERRUPT SIGNAL
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

	// TODO:
	// balance updater

	time.Sleep(time.Second)
	if all {
		if verbose {
			for asset := range MapAssets() {
				assets = append(assets, asset)
			}
		} else {
			for asset := range balance.Balances {
				//check if balance is worth more than mimimum
				free := balance.Balances[asset].Free * 0.999
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

	log.Println("assets", assets)

	mapSets()

	// create pairs to subcribe to
	var pairs []string
	for _, a := range assets {
		for _, fn := range arbs {
			pairs = append(pairs, fn.Sets(a).List()...)
		}
	}

	// pairs = []string{"BTCUSDT", "DOTBTC", "DOTUSDT", "LUNAUSDT", "LUNABTC"}
	// log.Printf("%s total: %d\n", pairs, len(pairs))
	pairs = pairs[:1000]
	log.Printf("connecting to %d pairs\n", len(pairs))

	// handle ws streams
	var handlarC = make(chan []byte, len(pairs))

	// handle orderbook data
	go func() {
		for {
			select {
			case <-shutdown:
				return

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

				// loop throu all possible routes
				pair, _ := E.Pair(resp.Symbol)
				sets := SetsMap[pair]

				// go func() ==>
				for _, set := range sets {
					for _, asset := range assets {
						if asset != set.asset {
							continue
						}
						// check if balance is worth more than mimimum
						free := balance.Balances[asset].Free * 0.999
						_pair, err := E.Pair(asset + "USDT")
						if err == nil {
							free *= tickers[_pair.Name]
						}
						if minimum > free {
							continue
						}

						// free := 300.

						// check is we can profit                           balance.Balances[asset].Free
						if amount1, amount2, amount3 := set.calcStepProfits(balance.Balances[asset].Free * 0.999); amount1 != 0 && amount2 != 0 && amount3 != 0 {
							// _ = amount1
							// _ = amount2
							// _ = amount3

							var err error

							log.Println(set.a.Name, actions[set.route[0]], amount1)
							minAmount := amount1 - (amount1 * MAKER_FEE)
							for amount1 > minAmount {
								if err = E.SendMarket(set.a.Name, actions[set.route[0]], amount1); err != nil {
									// dont decrease if not balance error
									amount1 -= (amount1 * 0.0001)
									time.Sleep(100 * time.Millisecond)
								}
								break
							}
							if err != nil {
								log.Fatalln("err1", err)
							}

							log.Println(set.b.Name, actions[set.route[1]], amount2)
							minAmount = amount2 - (amount2 * MAKER_FEE)
							for amount2 > minAmount {
								if err = E.SendMarket(set.b.Name, actions[set.route[1]], amount2); err != nil {
									amount2 -= (amount2 * 0.0001)
									time.Sleep(100 * time.Millisecond)
								}
								break
							}
							if err != nil {
								log.Fatalln("err2", err)
							}

							log.Println(set.c.Name, actions[set.route[2]], amount3)
							minAmount = amount3 - (amount3 * MAKER_FEE)
							for amount3 > minAmount {
								if err = E.SendMarket(set.c.Name, actions[set.route[2]], amount3); err != nil {
									amount3 -= (amount3 * 0.0001)
									time.Sleep(100 * time.Millisecond)
								}
								break
							}
							if err != nil {
								log.Fatalln("err3", err)
							}

							time.Sleep(5 * time.Second)

							close(shutdown)
						}
					}
				}

			default:
			}
		}
	}()

	// subscribe to orderbooks
	for _, pair := range pairs {
		go subscribePair(pair, handlarC)
	}

	<-shutdown
}

func reDo(fn func() error, amount float64) error {
	minAmount := amount
	minAmount -= (amount * MAKER_FEE)

	var err error
	for amount < minAmount {
		if err = fn(); err != nil {
			amount -= (amount * 0.0001)
			time.Sleep(10 * time.Millisecond)
		}
		// success
		return nil
	}
	// failed
	return err
}

func subscribePair(name string, handler chan<- []byte) {

restart:
	u := url.URL{
		Scheme: "wss",
		Host:   "stream.binance.com:9443",
		Path:   fmt.Sprintf("/ws/%s@depth%s", strings.ToLower(name), level),
	}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("dial:", err)
		return
	}
	c.PongHandler()
	log.Println("subscribed to", name)

	// go keepAlive(c, time.Minute)
	defer c.Close()

	for {
		select {
		case <-shutdown:
			return
		default:
		}

		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("ws closed. reconnecting", name)

			lock <- true
			orderbook.Delete(name)
			<-lock

			c.Close()
			goto restart
		}

		handler <- message
	}
}

func keepAlive(c *websocket.Conn, timeout time.Duration) {
	ticker := time.NewTicker(timeout)

	lastResponse := time.Now()
	c.SetPongHandler(func(msg string) error {
		lastResponse = time.Now()
		return nil
	})

	for {
		select {
		case <-shutdown:
			return

		case <-ticker.C:
			if time.Since(lastResponse) > timeout {
				log.Println("keepAlive timeout. closing.")
				c.Close()
				return
			}
			deadline := time.Now().Add(10 * time.Second)
			err := c.WriteControl(websocket.PingMessage, []byte{}, deadline)
			if err != nil {
				log.Println("keepAlive error:", err)
				c.Close()
				return
			}

		default:
		}
	}
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

// atomic
const free = int32(0)

var state *int32

func Spinlock(f func()) {
	for !atomic.CompareAndSwapInt32(state, free, 1) {
		runtime.Gosched()
	}
	f()
	defer atomic.StoreInt32(state, free)
}
