package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
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
	"github.com/slicken/arbitrager/utils"
)

var (
	// app arguments
	assets  []string
	except  []string
	all     bool          = false
	target  float64       = 1.5
	steps   int           = 1
	minimum float64       = 100.
	level   string        = ""
	cpu     int           = 0
	limit   int           = 1024
	verbose bool          = false
	debug   bool          = false
	sec     time.Duration = 60
	// app variables
	E          exchanges.I
	tickers    map[string]float64
	shutdown   = make(chan bool)
	_, appName = filepath.Split(os.Args[0])
	lastTrade  = time.Now()
)

func appInfo() {
	fmt.Println(`Usage: ./` + appName + ` [-a <assets>|--all] [-e <assets>] [-t <percent>] [-n <uint>] [-m <USD>]
             [--100] [--CPU <cores>] [--verbose] [-l <uint>] [--sec <uint>]
Arguments        Examples
  -a, --asset    BTC,ETH,BNB              only thease assets. TIP use quote assets
      --all                               all assets with balance
  -e, --except   DOT,USDC                 except thease assets
  -t, --target   0.99                     target percent             (default  1.5)
  -n, --decrease 2                        decrease balance N times   (default    1)
  -m, --minimum  50                       mimimum balance (in USD)   (default  100)
  -l, --limit    300                      limit orderbooks           (default 1024)
      --sec      180                      paus to next trade is sec  (default   60)
      --CPU      2                        limit cpu cores            (default  max)
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
				v, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo()
				}
				target = v
				log.Println("target percent", target)

			case "-n", "--decrease":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo()
				}
				steps = v
				log.Printf("decrease balance %d times\n", steps)

			case "-m", "--minimum":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo()
				}
				minimum = v
				log.Println("minimum balance (in USD)", minimum)

			case "-l", "--limit":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo()
				}
				limit = v
				log.Println("limit orderbooks to", v)

			case "--sec":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo()
				}
				sec = time.Duration(v)
				log.Printf("order sleep set to %dsec\n", v)

			case "--100":
				level = "20@100ms"
				log.Println("fetching data every 100ms")

			case "--CPU":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo()
				}
				cpu = v
				log.Println("cpu cores limited to", cpu)

			case "-h", "--help", "help":
				appInfo()

			case "--verbose":
				verbose = true
				log.Println("verbose enabled")

			case "--debug":
				debug = true
				log.Println("debug enabled")

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
	utils.LogToFile(appName)

	// LIMIT CPU CORES
	if cpu != 0 {
		runtime.GOMAXPROCS(cpu)
	}
	// LOAD CONFIG FILE
	if err := config.ReadConfig(); err != nil {
		log.Fatalln("could not load config file:", err.Error())
	}
	log.Println("reading config...")

	// LOAD EXCHANGE
	if err := LoadExchange("binance"); err != nil {
		log.Fatalln("could not load exchange:", err.Error())
	}
	log.Println("connected to", E.GetName())

	time.Sleep(time.Second)

	// PREPARE DATA ---------------------------------

	// download tickers
	var err error
	if tickers, err = E.GetAllTickers(); err != nil {
		log.Fatalln("failed to update tickers:", err.Error())
	}
	_ = err

	if all {
		for asset := range balance.Balances {
			free := balance.Balances[asset].Free * 0.99
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
	for _, e := range except {
		for i, t := range assets {
			if e == t {
				assets = append(assets[:i], assets[i+1:]...)
			}
		}
	}

	log.Println("found assets", assets)

	mapSets()
	pairs := SetMapList()

	for _, e := range except {
		for i, p := range pairs {
			v, _ := E.Pair(p)
			if e == v.Base || e == v.Quote {
				pairs = append(pairs[:i], pairs[i+1:]...)
			}
		}
	}

	if len(pairs) > limit {
		pairs = pairs[:limit]
	}

	log.Printf("connecting to %d orderbooks\n%s", len(pairs), pairs)

	// update tickers
	ticker := time.NewTicker(time.Hour)
	// handle ws streams
	var handlarC = make(chan []byte, len(pairs))
	var orderC = make(chan OrderSet, 1)

	// handle channel events ----------------------------------
	// 	- update orderbook
	//  - send order
	//  - update tickers & balance
	//
	go func() {
		for {
			select {
			case <-shutdown:
				return

			case <-ticker.C:
				var err error
				if tickers, err = E.GetAllTickers(); err != nil {
					log.Println("failed to update tickers:", err.Error())
				}
				if err := E.UpdateBalance(); err != nil {
					log.Println("failed to uodate balance:", err.Error())
				}

			case order := <-orderC:
				var err error
				var _pair = [3]string{order.a.Name, order.b.Name, order.c.Name}
				var _amount = [3]float64{order.next1, order.next2, order.next3}
				// var amount = _amount
				for i, side := range order.route {

					// if we bought at a worse price than intended
					// we can remove diff from amount
					// if i > 0 {
					// 	amount[i] += ((amount[i] - _amount[i]) / amount[i])
					// }

					tries := 0
					minAmount := _amount[i] - (_amount[i] * 0.05) //(3 * MAKER_FEE))
					for _amount[i] > minAmount {
						// send market order
						log.Printf("%-6s %-12v     %-12s\n", Side[side], _amount[i], _pair[i])
						err = E.SendMarket(_pair[i], Side[side], _amount[i])
						if err == nil {
							break
						}
						// on balance error - decrease amount a bit
						if !containList(err.Error(), []string{"dial tcp", "too many"}) {
							_amount[i] -= (_amount[i] * 0.001) //02)
						} else if tries > 1 || i == 0 {
							// return if first trade
							log.Println("ERROR:", err.Error())
							lastTrade = time.Now().Add(sec * time.Second)
							return
						} else {
							tries++
						}
						log.Println("ERROR:", err.Error())
					}
					// if we still have error - close program
					if err != nil {
						log.Fatalln(err.Error())
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
					log.Println("failed to update balance:", err.Error())
					time.Sleep(delay)
					delay *= 3
					tries++
				}
				if err != nil {
					log.Fatalln(err.Error())
				}
				// success! dont look for new trades for x minutes
				lastTrade = time.Now().Add(sec * time.Second)

			case b := <-handlarC:
				var resp WsDepthEvent
				if err := json.Unmarshal(b, &resp); err != nil {
					log.Println(err.Error())
					continue
				}

				book, _ := orderbook.GetBook(resp.Symbol)
				book.LastUpdated = time.Now()
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

				// continue if to early
				if time.Now().Before(lastTrade) {
					continue
				}

				// loop throu all possible routes
				pair, _ := E.Pair(resp.Symbol)
				sets := SetsMap[pair]

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

						// // ------- make it more concurrent --->
						// // TODO:  add lockless or mutex.
						//
						// go func(s Set) {
						// if order := set.calcStepProfits(300); order != nil {
						// 		orderC <- order
						// 	}

						// }(set)
						// // -------------------------------------

						if order := set.calcStepProfits(100); order != nil {

							var err error
							var _pair = [3]string{order.a.Name, order.b.Name, order.c.Name}
							var _amount = [3]float64{order.next1, order.next2, order.next3}
							// var amount = _amount
							for i, side := range order.route {

								// if we bought at a worse price than intended
								// we can remove diff from amount
								// if i > 0 {
								// 	amount[i] += ((amount[i] - _amount[i]) / amount[i])
								// }

								tries := 0
								minAmount := _amount[i] - (_amount[i] * 0.05) // 5% smaller
								// retry func
								for _amount[i] > minAmount {
									// send market order
									log.Printf("%-6s %-12v     %-12s\n", Side[side], _amount[i], _pair[i])
									err = E.SendMarket(_pair[i], Side[side], _amount[i])
									if err == nil {
										break
									}
									// on balance error - decrease amount a bit
									if containList(err.Error(), []string{"dial tcp", "too many"}) {
										if i == 0 || tries > 9 {
											log.Println("ERROR:", err.Error())
											lastTrade = time.Now().Add(sec * time.Second)
											return
										}
										tries++
									} else {
										// decrease amount and try again
										_amount[i] -= (_amount[i] * 0.001)
									}
									log.Println("ERROR:", err.Error())
								}
								// exit program if we get here. > 10 errors
								if err != nil {
									log.Fatalln(err.Error())
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
								log.Println("ERROR:", err.Error())
								time.Sleep(delay)
								delay *= 3
								tries++
							}
							if err != nil {
								log.Fatalln(err.Error())
							}
							// success! dont look for new trades for x minutes
							lastTrade = time.Now().Add(sec * time.Second)
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

	log.Println("init done! running...")

	<-shutdown
}

func subscribePair(name string, handler chan<- []byte) {
	u := url.URL{
		Scheme: "wss",
		Host:   "stream.binance.com:9443",
		Path:   fmt.Sprintf("/ws/%s@depth%s", strings.ToLower(name), level),
	}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("dial:", err.Error())
		return
	}
	defer c.Close()
	if verbose {
		log.Println("subscribed to", name)
	}

	// go keepAlive(c, time.Minute)

	for {
		select {
		case <-shutdown:
			return
		default:
		}

		_, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("reconnecting %s due to: %v", name, err.Error())
			orderbook.Delete(name)
			go subscribePair(name, handler)
			break
		}

		handler <- message
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

// func keepAlive(c *websocket.Conn, timeout time.Duration) {
// 	ticker := time.NewTicker(timeout)

// 	lastResponse := time.Now()
// 	c.SetPongHandler(func(msg string) error {
// 		lastResponse = time.Now()
// 		return nil
// 	})

// 	for {
// 		select {
// 		case <-shutdown:
// 			return

// 		case <-ticker.C:
// 			if time.Since(lastResponse) > timeout {
// 				log.Println("keepAlive timeout. closing.")
// 				c.Close()
// 				return
// 			}
// 			deadline := time.Now().Add(10 * time.Second)
// 			err := c.WriteControl(websocket.PingMessage, []byte{}, deadline)
// 			if err != nil {
// 				log.Println("keepAlive ERROR:", err.Error())
// 				c.Close()
// 				return
// 			}

// 		default:
// 		}
// 	}
// }
