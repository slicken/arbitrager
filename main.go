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
	target   float64 = 2.0
	steps    int     = 1
	size     float64 = 100
	minimum  float64 = 20
	download bool    = false
	obdiff   bool    = false
	cpu      int     = 0
	limit    int     = 1024
	verbose  bool    = false
	debug    bool    = false
	// app variables - dont change
	E          exchanges.I
	tickers    map[string]float64
	shutdown   = make(chan bool)
	_, appName = filepath.Split(os.Args[0])
	lastTrade  = time.Now()
)

func appInfo(code int) {
	fmt.Println(`Usage: ./` + appName + ` [-a <assets>|--all] [-e <assets>] [-t <percent>] [-n <uint>] [-m <USD>]
             [--100] [--CPU <cores>] [--verbose] [-l <uint>] [--sec <uint>]

Arguments       Default   Example   Info
  -a, --asset             USDT,BTC  enter assets to arbitrage. separateor ',' if more than one
      --all                         arbitrage all assets with a balance
  -e, --except            USDC      except thease assets
  -t, --target    2.0     1.7       minimum target in percentage to trade
  -s, --size      100     500       tradesize mearesured in USD. (0=blanace)
  -n, --decrease          2         also look for arbitrages with a decrease balance N times
  -l, --limit     1024              limit maximum connections to orderbooks
      --diff      false             streams orderbook diffs (1sec) instead of snapshots (100ms)
      --download  max     2         downloads orderbook, for '--diff' mode only
      --CPU                         limit usage of cpu cores
      --verbose
  -h  --help
                                       -- slk prod 2021 --`)
	os.Exit(code)
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
		appInfo(1)
	} else {
		for i, arg := range os.Args[1:] {
			switch arg {
			case "-a", "--asset":
				if i+3 > len(os.Args) {
					appInfo(1)
				}
				assets = Split(os.Args[i+2])

			case "--all":
				all = true

			case "-e", "--except":
				if i+3 > len(os.Args) {
					appInfo(1)
				}
				except = Split(os.Args[i+2])

			case "-t", "--target":
				if i+3 > len(os.Args) {
					appInfo(1)
				}
				v, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo(1)
				}
				target = v

			case "-n", "--decrease":
				if i+3 > len(os.Args) {
					appInfo(1)
				}
				v, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo(1)
				}
				steps = v
				log.Printf("decrease balance %d times\n", steps)

			case "-s", "--size":
				if i+3 > len(os.Args) {
					appInfo(1)
				}
				v, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo(1)
				}
				size = v
				log.Println("tradesize (in USD)", size)

			case "-m", "--minimum":
				if i+3 > len(os.Args) {
					appInfo(1)
				}
				v, err := strconv.ParseFloat(os.Args[i+2], 64)
				if err != nil {
					appInfo(1)
				}
				minimum = v
				log.Println("min tradesize (in USD)", minimum)

			case "--download":
				download = true
				log.Println("dowloading orderbooks")

			case "-l", "--limit":
				if i+3 > len(os.Args) {
					appInfo(1)
				}
				v, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo(1)
				}
				limit = v
				log.Println("limit orderbooks to", limit)

			case "--diff":
				obdiff = true
				log.Println("orderbook diff enabled (1sec update)")

			case "--CPU":
				if i+3 > len(os.Args) {
					appInfo(1)
				}
				v, err := strconv.Atoi(os.Args[i+2])
				if err != nil {
					appInfo(1)
				}
				cpu = v
				log.Println("cpu cores limited to", cpu)

			case "-h", "--h", "-help", "--help", "help":
				appInfo(0)

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
		appInfo(1)
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

	// ---- TEST ------------------------------------------------------

	// // buy	ATMUSDT		amount
	// // sell	ATMBUSD		amount
	// // sell	BUSDUSDT	amount

	// qty, er := E.SendMarket("ERNBUSD", "BUY", 0, 20)
	// if er != nil {
	// 	log.Fatalln(er.Error())
	// }
	// fmt.Printf("RESULT >> %.8f\n", qty)
	// os.Exit(0)

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
			if size > 0 && size < free {
				free = size
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
			// check arbitrage opportunities
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
						if size > 0 && size < free {
							free = size
						}
						if minimum > free {
							continue
						}

						// TODO:
						// make this concurrent?
						//
						if o := set.calcStepProfits(size); o != nil {
							if time.Now().Before(lastTrade) {
								continue
							}
							lastTrade = time.Now().Add(30 * time.Second)

							qty := o.initial
							var resp float64
							var msg string
							var err error
							for i, side := range o.route {

								tries := 0
								for 5 > tries {
									// prepare order message
									if tries == 0 {
										msg = fmt.Sprintf("%-6s %-12f     %-12s  %%s\n", Side[side], qty, o.pair[i].Name)
									} else {
										msg = fmt.Sprintf("%-6s %-12f     %-12s  %%s\n", "RETRY", qty, o.pair[i].Name)
									}
									if side == 0 {
										resp, err = E.SendMarket(o.pair[i].Name, Side[side], 0, qty)
									} else {
										resp, err = E.SendMarket(o.pair[i].Name, Side[side], qty, 0)
									}
									if resp != 0 {
										qty = resp
									}
									if err == nil {
										if i != 2 {
											log.Printf(msg, "ok")
										}
										break
									}

									if i == 0 {
										lastTrade = time.Now().Add(5 * time.Minute)
										log.Printf(msg, "fail         skipping trade. we failed to create o and now it would be to late, cause this is time sensitive.")
										return
									}
									tries++
									log.Printf(msg, "fail  error: "+err.Error())
								}
								// exit program if we get here. > 5 tries
								if err != nil {
									log.Printf(msg, "fail         exiting too many tries")
									log.Fatalln(err.Error())
								}
							}
							// final results here
							log.Printf(msg, fmt.Sprintf("%f (%5.2f%%)", qty-o.initial, (qty/o.initial)*100-100))

							// update balance
							log.Println("updating balance...")
							tries := 0
							delay := 100 * time.Microsecond
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
							lastTrade = time.Now().Add(5 * time.Minute)
						}
					}
				}

			//
			// send orders
			//
			case o := <-orderC:
				log.Println("<-orderC", o)
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

/*

	if o := set.calcStepProfits(size); o != nil {
		if time.Now().Before(lastTrade) {
			continue
		}
		lastTrade = time.Now().Add(30 * time.Second)

		var p, q float64
		var msg string
		var err error
		for i, side := range o.route {

			lt := false
			tries := 0
			for 5 > tries {
				// prepare order message
				if tries == 0 {
					msg = fmt.Sprintf("%-6s %-12f     %-12s  %%s\n", Side[side], o.amount[i], o.pair[i].Name)
				} else {
					msg = fmt.Sprintf("%-6s %-12f     %-12s  %%s\n", "RETRY", o.amount[i], o.pair[i].Name)
				}

				if i > 0 {
					if q != 0 {
						o.amount[i] = q
					}
				}

				p, q, err = E.SendMarket(o.pair[i].Name, Side[side], o.amount[i], 0)
				if err == nil {
					log.Printf("price %f\n", p)
					log.Printf("qty %f\n", q)
					if i != 2 {
						log.Printf(msg, "ok")
					}
					break
				}

				tries++
				if containList(err.Error(), []string{"dial tcp", "too many"}) {
					if i == 0 {
						lastTrade = time.Now().Add(5 * time.Minute)
						log.Printf(msg, "fail         skipping trade. we failed to create o and now it would be to late, cause this is time sensitive.")
						return
					}
					log.Printf(msg, "fail  error: "+err.Error())
					continue
				}
				if i > 0 {
					log.Printf(msg, "fail")
					if lt {
						continue
					}

					continue

					// TODO:
					// fix calculation
					// BBS & SBB is disabled due to  calculations error.. whats best in theae ..
					if price, amount, qAmount, fee, err := E.LastTrade(o.pair[i-1].Name, 10); err == nil {
						lt = true
						switch side {
						case 0:
							// i dont know how this
							price := amount / o.amount[i]
							o.amount[i] = (amount / price) - (MAKER_FEE * (amount / price))
							// o.amount[i] = amount / price
						case 1:
							// exact amount will fail so remove a0.01%
							if i == 1 {
								o.amount[i] = amount - (MAKER_FEE * amount)
							} else {
								o.amount[i] = qAmount - fee
							}
						}
						_ = price
					}
				}
			}
			// exit program if we get here. > 5 tries
			if err != nil {
				log.Printf(msg, "fail         exiting too many tries")
				log.Fatalln(err.Error())
			}
		}
		// results here
		if price, _, _, fee, err := E.LastTrade(o.pair[2].Name, 10); err == nil {
			final := (price * o.amount[2]) - fee
			perc := (final/o.initial)*100 - 100
			log.Printf(msg, fmt.Sprintf("%f (%5.2f%%)", final-o.initial, perc))
		} else {
			log.Printf(msg, "ok")
		}

		// update balance
		log.Println("updating balance...")
		tries := 0
		delay := 100 * time.Microsecond
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
		lastTrade = time.Now().Add(5 * time.Minute)
	}

*/
