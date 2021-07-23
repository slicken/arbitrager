package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/slicken/arbitrager/balance"
	"github.com/slicken/arbitrager/config"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/utils"
)

var (
	// app arguments
	assets   []string
	except   []string
	all      bool    = false
	target   float64 = 1.5
	steps    int     = 1
	size     float64 = 100
	minimum  float64 = 20
	download bool    = false
	obdiff   bool    = false
	cpu      int     = 0
	limit    int     = 200
	verbose  bool    = false
	debug    bool    = false
	// app variables - dont change
	E          exchanges.I
	tickers    map[string]float64
	shutdown   = make(chan bool)
	_, appName = filepath.Split(os.Args[0])
	lastTrade  = time.Now()
)

func appInfo() {
	fmt.Println(`Usage: ./` + appName + ` [-a <assets>|--all] [-e <assets>] [-t <percent>] [-n <uint>] [-m <USD>]
             [--100] [--CPU <cores>] [--verbose] [-l <uint>] [--sec <uint>]

Arguments       Default   Example   Info
  -a, --asset             USDT,BTC  enter assets to arbitrage. separateor ',' if more than one
      --all                         arbitrage all assets with a balance
  -e, --except            USDC      except thease assets
  -t, --target    1.5     2         minimum target in percentage to trade
  -s, --size      500     100       tradesize mearesured in USD
  -n, --decrease  1024    2         also look for arbitrages with a decrease balance N times
  -l, --limit     false             limit maximum connections to orderbooks
      --diff      false             streams orderbook diffs (1s) instead of snapshots (100ms)
      --download  max     2         downloads orderbook, for '--diff' mode only
      --CPU                         limit usage of cpu cores
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

			case "-e", "--except":
				if i+3 > len(os.Args) {
					appInfo()
				}
				except = Split(os.Args[i+2])

			case "-t", "--target":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo()
				}
				target = v

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

			case "-s", "--size":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo()
				}
				size = v
				log.Println("tradesize (in USD)", size)

			case "-m", "--minimum":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo()
				}
				minimum = v
				log.Println("min tradesize (in USD)", minimum)

			case "--download":
				download = true
				log.Println("dowloading orderbooks")

			case "-l", "--limit":
				if i+3 > len(os.Args) {
					appInfo()
				}
				v, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo()
				}
				limit = v
				log.Println("limit orderbooks to", limit)

			case "--diff":
				obdiff = true
				log.Println("orderbook diff enabled (1sec update)")

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
	log.Printf("target is %.2f%%\n", target)

	// HANDLE INTERRUPT SIGNAL
	HandleInterrupt()

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

	// LOG TO FILE
	utils.LogToFile(appName)

	// PREPARE DATA ---------------------------------

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

	if len(assets) == 0 {
		log.Fatalln("no assets found")
	}
	if all {
		log.Println("found assets", assets)
	} else {
		log.Println("assets", assets)
	}

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

	log.Printf("connecting to %d orderbooks --> %s", len(pairs), pairs)

	// auto updates
	updates := time.NewTicker(time.Hour)
	// handler channels
	var checkC = make(chan string, len(pairs))
	var orderC = make(chan OrderSet, 1)

	go func() {
		for {
			select {
			case <-shutdown:
				return
			//
			// update tickers and balance
			//
			case <-updates.C:
				var err error
				if tickers, err = E.GetAllTickers(); err != nil {
					log.Println("failed to update tickers:", err.Error())
				}
				if err := E.UpdateBalance(); err != nil {
					log.Println("failed to uodate balance:", err.Error())
				}
			//
			// computeC
			//
			case name := <-checkC:
				// continue if to early
				if time.Now().Before(lastTrade) {
					continue
				}

				// loop throu all possible routes
				pair, _ := E.Pair(name)
				sets := SetsMap[pair]
				for _, set := range sets {
					for _, asset := range assets {
						if asset != set.asset {
							continue
						}

						free := balance.Balances[asset].Free * 0.9
						_pair, err := E.Pair(asset + "USDT")
						if err == nil {
							free *= tickers[_pair.Name]
						}
						if size < free {
							free = size
						}
						if minimum > free {
							continue
						}

						if order := set.calcStepProfits(size); order != nil {
							// fmt.Println("orderC <-", order)
							orderC <- *order
						}
					}
				}

			//
			// send orders
			//
			case order := <-orderC:

				var err error
				var _pair = [3]string{order.a.Name, order.b.Name, order.c.Name}
				var _amount = [3]float64{order.next1, order.next2, order.next3}
				// var amount = _amount
				for i, side := range order.route {
					tries := 0

					for 5 > tries {
						// send market order
						log.Printf("%-12s %-12v %-12s\n", Side[side], _amount[i], _pair[i])
						err = E.SendMarket(_pair[i], Side[side], _amount[i])
						if err == nil {
							break
						}
						if containList(err.Error(), []string{"dial tcp", "too many"}) {
							if i == 0 || tries > 5 {
								log.Println("ERROR:", err.Error())
								lastTrade = time.Now().Add(time.Minute)
								return
							}
						}
						tries++
						if i > 0 {
							tries2 := 0
							for 5 > tries2 {
								if _, a, err := E.LastTrade(_pair[i-1], 5); err == nil {
									_amount[i] = a
									break
								}
								log.Println("ERROR:", err.Error())
								tries2++
							}
						}
					}
					// exit program if we get here. > 5 tries
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
				// success! paus trading for a minute
				lastTrade = time.Now().Add(time.Minute)

			default:
			}
		}
	}()

	// subscribe to orderbooks
	for _, pair := range pairs {

		if obdiff {
			go E.StreamBookDiff(pair, shutdown, checkC)
		} else {
			go E.StreamBookDepth(pair, shutdown, checkC)

		}

	}

	log.Println("-------------------- init done!")
	log.Println("running...")

	<-shutdown
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
