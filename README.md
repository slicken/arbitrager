[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square)](/LICENSE.md)


# arbitrager

connecting up to 1024 orderbooks via websocket<br>
and scans for arbitrage opportunities on<br>
varoius popular exchanges.
<br><br>

### STRATEGIES SUPPORTED

Triangular arbitrage - *checking all possible trade routes*
<br><br>

### EXCHANGES SUPPORTED

Binance
<br><br>


# TODO:
- [x] stream orderbook snapshots default (100ms)
- [ ] better order function
  - [x] retry function
  - [ ] get last trade info
- [ ] Make orderbook lockless
- [ ] Kucoin exchange support
- [ ] Buy and Sell cross exchange strategy
- [ ] Buy and sell futures market strategy
- [ ] FTX exchange support
- [ ] more exchange supports...
- [ ] Optimize preformance
- [ ] and more...
<br><br>


# Previev
```
Usage: ./app [-a <assets>|--all] [-e <assets>] [-t <percent>] [-n <uint>] [-m <USD>]
             [--100] [--CPU <cores>] [--verbose] [-l <uint>] [--sec <uint>]

Arguments       Default   Example   Info
  -a, --asset             USDT,BTC  enter assets to arbitrage. separateor ',' if more than one
      --all                         arbitrage all assets with a balance
  -e, --except            USDC      except thease assets
  -t, --target    1.5     2         minimum target in percentage to trade
 
 # TODO:

*Make orderbook lockless<br>
Optimize preformance<br>
add Kucoin exchange support<br>
add FTX exchange support<br>
and more exchange supports...<br>
Buy, Sell cross exchange strategy*<br><br>


s, --size      500     100       tradesize mearesured in USD
  -n, --decrease  1024    2         also look for arbitrages with a decrease balance N times
  -l, --limit     false             limit maximum connections to orderbooks
      --diff      false             streams orderbook diffs (1s) instead of snapshots (100ms)
      --download  max     2         downloads orderbook, for '--diff' mode only
      --CPU                         limit usage of cpu cores
      --verbose
  -h  --help
                                       -- slk prod 2021 --

slicken@slk:~/go/src/github.com/slicken/arbitrager$ ./app -a USDT -t 1 -s 100 -l 200 --verbose
2021/07/23 16:03:17 tradesize (in USD) 100
2021/07/23 16:03:17 limit orderbooks to 200
2021/07/23 16:03:17 verbose enabled
2021/07/23 16:03:17 trade target is 1.00%
2021/07/23 16:03:17 reading config...
2021/07/23 16:03:18 connected to binance
2021/07/23 16:03:18 logging to "app_20210723.log"
2021/07/23 16:03:18 assets [USDT]
2021/07/23 16:03:19 connecting to 200 orderbooks --> [PAXUSDT BNBPAX BNBUSDT ETHPAX ETHUSDT BTCPAX BTCUSDT PAXBUSD BUSDUSDT TWTUSDT TWTBUSD TWTBTC IOSTUSDT IOSTETH IOSTBNB IOSTBUSD IOSTBTC ALGOUSDT ALGOBUSD ALGOBNB ALGOBTC BTTUSDT BTTBNB BTTBUSD BTTUSDC USDCUSDT BTTTUSD TUSDUSDT BTTEUR EURUSDT BTTTRX TRXUSDT LINKUSDT LINKETH LINKUSDC LINKGBP GBPUSDT LINKEUR LINKTUSD LINKAUD AUDUSDT LINKBTC LINKBUSD RVNUSDT RVNBUSD RVNBTC RVNBNB ZECUSDT ZECUSDC ZECBUSD ZECBTC ZECETH ZECBNB TLMUSDT TLMBTC TLMBUSD DOTUSDT DOTGBP DOTEUR DOTBNB DOTBUSD DOTAUD DOTBTC MIRUSDT MIRBUSD MIRBTC CVCUSDT CVCBTC CVCETH DOCKUSDT DOCKBUSD DOCKBTC PAXGUSDT PAXGBTC PAXGBNB ALPHAUSDT ALPHABTC ALPHABUSD ALPHABNB KSMUSDT KSMBUSD KSMBTC KSMBNB UMAUSDT UMABTC BADGERUSDT BADGERBTC BADGERBUSD HBARUSDT HBARBUSD HBARBNB HBARBTC RENUSDT RENBTC FETUSDT FETBNB FETBTC LPTUSDT LPTBUSD LPTBTC LPTBNB SFPUSDT SFPBUSD SFPBTC SANDUSDT SANDBNB SANDBUSD SANDBTC UNIUSDT UNIBTC UNIBNB UNIBUSD UNIEUR BANDUSDT BANDBUSD BANDBTC BANDBNB SXPUSDT SXPBNB SXPAUD SXPGBP SXPBTC SXPEUR SXPBUSD PNTUSDT PNTBTC NUUSDT NUBNB NUBTC NUBUSD CAKEUSDT CAKEBNB CAKEBUSD CAKEGBP CAKEBTC TKOUSDT TKOBTC TKOBUSD SUSHIUSDT SUSHIBUSD SUSHIBNB SUSHIBTC KNCUSDT KNCBTC KNCETH KNCBUSD HIVEUSDT HIVEBTC MATICUSDT MATICGBP MATICBTC MATICBUSD MATICEUR MATICBNB MATICAUD IOTAUSDT IOTABTC IOTABUSD IOTAETH IOTABNB XLMUSDT XLMETH XLMBNB XLMBUSD XLMBTC XLMEUR ONGUSDT ONGBTC STPTUSDT STPTBTC STRAXUSDT STRAXBTC STRAXETH STRAXBUSD WINUSDT WINEUR WINBNB WINBUSD WINUSDC WINTRX GTCUSDT GTCBNB GTCBTC GTCBUSD ARDRUSDT ARDRBTC PERLUSDT PERLBNB PERLBTC VETUSDT VETBUSD VETETH VETEUR VETGBP VETBTC VETBNB WTCUSDT WTCBTC MKRUSDT MKRBNB]
2021/07/23 16:03:19 -------------------- init done!
2021/07/23 16:03:19 running...
2021/07/23 16:31:56 USDT   100.000000       0.011790     ( 0.01%)      buy BTCUSDT      32539.280000      buy SANDBTC      0.000018         sell SANDUSDT     0.598000    
2021/07/23 16:33:37 USDT   100.000000       0.015029     ( 0.02%)      buy BNBUSDT      291.460000        buy LPTBNB       0.039810         sell LPTUSDT      11.640000   
2021/07/23 16:38:23 USDT   100.000000       0.127378     ( 0.13%)      buy BANDUSDT     5.086000         sell BANDBNB      0.017630         sell BNBUSDT      289.730000  
```

