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

	lock      = make(chan bool, 1)
	lastTrade = time.Now().Add(10 * time.Minute)
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
				// log.Println("arbitraging", assets)

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
	// log.Printf("%s total: %d\n", pairs, len(pairs))
	pairs = pairs[:1000]
	log.Printf("connecting to %d pairs...\n", len(pairs))

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

				// to early =
				if time.Now().Before(lastTrade) {
					continue
				}
				// TODO:
				// go func() ==> make it concurrent here
				// spinlock or lockless @ ws read part
				//
				for _, set := range sets {
					for _, asset := range assets {
						if asset != set.asset {
							continue
						}
						// check if balance is worth more than mimimum
						free := balance.Balances[asset].Free * 0.9
						_pair, err := E.Pair(asset + "USDT")
						if err == nil {
							free *= tickers[_pair.Name]
						}
						if minimum > free {
							continue
						}
						// does traderoute make profit
						if amount1, amount2, amount3 := set.calcStepProfits(balance.Balances[asset].Free * 0.9); amount1 != 0 && amount2 != 0 && amount3 != 0 {

							var err error
							var _amount = [3]float64{amount1, amount2, amount3}
							var _pair = [3]string{set.a.Name, set.b.Name, set.c.Name}
							for i, _action := range set.route {

								minAmount := _amount[i] - (_amount[i] * (3 * MAKER_FEE))
								for _amount[i] > minAmount {

									log.Printf("-->   %-6s %-12f %-12s\n", actions[_action], _amount[i], _pair[i])
									err = E.SendMarket(_pair[i], actions[_action], _amount[i])
									if err == nil {
										break
									}
									// try decrease amount a bit
									if containList(err.Error(), []string{"balance", "quantity"}) {
										_amount[i] -= (_amount[i] * 0.0002)
									} else {
										// if we havent made any trades then we can pass and look for new oppertunities
										if i == 0 {
											log.Println("error:", err)
											lastTrade = time.Now().Add(time.Minute)
											return
										}
									}
									log.Println("error:", err)
									time.Sleep(50 * time.Millisecond)
								}
								// if we still have error - close program
								if err != nil {
									log.Fatalln(err)
								}

							}
							// update balance
							tries := 0
							delay := 100 * time.Microsecond
							log.Println("updating balance...")
							for 5 > tries {
								if err = E.UpdateBalance(); err == nil {
									break
								}
								log.Println("error:", err)
								time.Sleep(delay)
								delay *= 3
								tries++
							}
							if err != nil {
								log.Fatalln(err)
							}
							// success! dont look for new trades for x minutes
							lastTrade = time.Now().Add(time.Minute)
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

func retryOrder(amount float64, attempts int, fn func() error) error {
	min := amount
	min -= (amount * MAKER_FEE)
	n := 0

	var err error
	for amount < min {

		err = fn()
		if err == nil {
			// success
			return nil
		}

		if strings.Contains(err.Error(), "balance") {
			amount -= (amount * 0.0002)
		} else {
			n++
		}
		if n > attempts {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// failed
	return err
}

func containList(str string, list []string) bool {
	for _, s := range list {
		if strings.Contains(str, s) {
			return true
		}
	}
	return false
}
