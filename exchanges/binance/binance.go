package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/slicken/arbitrager/balance"
	"github.com/slicken/arbitrager/client"
	"github.com/slicken/arbitrager/config"
	"github.com/slicken/arbitrager/currencie"
	"github.com/slicken/arbitrager/exchanges"
	"github.com/slicken/arbitrager/orderbook"
	"github.com/slicken/arbitrager/orders"
	"github.com/slicken/arbitrager/utils"
	"github.com/slicken/history"
)

const (
	apiURL = "https://api.binance.com"
	wsURL  = "wss://stream.binance.com:9443/ws"

	exchangeInfo = "/api/v1/exchangeInfo"
	account      = "/api/v3/account"
	depth        = "/api/v3/depth"
	ticker       = "/api/v3/ticker/price"
	tickerAll    = "/api/v1/ticker/allPrices"
	tickerBook   = "/api/v3/ticker/bookTicker"
	newOrder     = "/api/v3/order"

	maxRate = 1200
)

// Binance is exchange wrapper
type Binance struct {
	exchanges.Exchange
	Debug bool
	// *client.RateLimit
}

var info ExchangeInfo

// Setup takes in exchange configuration and sets params
func (e *Binance) Init(c config.ExchangeConfig) error {
	e.Name = c.Name
	e.Key = c.Key
	e.Secret = c.Secret
	e.Pairs = make(map[string]currencie.Pair)
	e.Requester = client.NewRequester(e.Name, client.NewHTTPClient(client.DefaultHTTPTimeout))
	e.Requester.Debug = false
	// test
	// e.RateLimit = client.NewRateLimit(1200)
	if err := e.SetPairs(); err != nil {
		return err
	}
	if err := e.UpdateBalance(); err != nil {
		return err
	}
	e.Enabled = true
	return nil
}

// SetPairs adds all symbols to exchange memory
func (e *Binance) SetPairs() error {
	info, err := e.ExchangeInfo()
	if err != nil {
		return err
	}

	e.Pairs = make(map[string]currencie.Pair)
	for _, v := range info.Symbols {
		p := currencie.Pair{}
		p.Name = v.Symbol
		p.Base = v.BaseAsset
		p.BaseDecimal = v.BaseAssetPrecision
		p.Quote = v.QuoteAsset
		p.QuoteDecimal = v.QuotePrecision
		if v.Status == "TRADING" {
			p.Enabled = true
		} else {
			p.Enabled = false
		}
		e.Pairs[p.Pair()] = p
	}

	return nil
}

// ExchangeInfo returns exchange information.
func (e *Binance) ExchangeInfo() (ExchangeInfo, error) {
	info = ExchangeInfo{}
	url := apiURL + exchangeInfo
	return info, e.SendHTTPRequest("GET", url, false, &info)
}

// getInfoIndex returns index position of symbol
func getInfoIndex(sym string) int {
	for i := range info.Symbols {
		if sym == info.Symbols[i].Symbol {
			return i
		}
	}
	return -1
}

// AccountBalance returns account balance
func (e *Binance) AccountBalance() (Account, error) {
	resp := Account{}
	url := apiURL + account
	return resp, e.SendHTTPRequest("GET", url, true, &resp)
}

// Ticker returns latest price of symbol
func (e *Binance) Ticker(symbol string) (SymbolPrice, error) {
	resp := SymbolPrice{}
	params := url.Values{}
	params.Set("symbol", symbol)
	url := fmt.Sprintf("%s%s?%s", apiURL, ticker, params.Encode())
	return resp, e.SendHTTPRequest("GET", url, false, &resp)
}

// TickerAll returns latest price of symbol
func (e *Binance) TickerAll() ([]SymbolPrice, error) {
	resp := []SymbolPrice{}
	url := apiURL + tickerAll
	return resp, e.SendHTTPRequest("GET", url, false, &resp)
}

// TickerBook returns best price of symbol
func (e *Binance) TickerBook(symbol string) (BookTicker, error) {
	resp := BookTicker{}
	params := url.Values{}
	params.Set("symbol", symbol)
	url := fmt.Sprintf("%s%s?%s", apiURL, tickerBook, params.Encode())
	return resp, e.SendHTTPRequest("GET", url, false, &resp)
}

// SendHTTPRequest sends the auth or unauth api request to exchange
func (e *Binance) SendHTTPRequest(method, url string, auth bool, result interface{}) error {
	req, err := http.NewRequest(method, url, strings.NewReader(""))
	if err != nil {
		return err
	}
	req.Header.Add("Accept", "application/json")

	if auth {
		if len(e.Key) == 0 || len(e.Secret) == 0 {
			return fmt.Errorf("Private endpoints requre you to set an API Key and API Secret")
		}
		req.Header.Add("X-MBX-APIKEY", e.Key)
		q := req.URL.Query()
		q.Set("timestamp", fmt.Sprintf("%d", time.Now().Unix()*1000))
		mac := hmac.New(sha256.New, []byte(e.Secret))
		_, err := mac.Write([]byte(q.Encode()))
		if err != nil {
			return err
		}
		signature := hex.EncodeToString(mac.Sum(nil))
		req.URL.RawQuery = q.Encode() + "&signature=" + signature
	}
	return e.Do(req, method, url, auth, &result)
}

// GetOrderbook Wrapper updates and returns the orderbook for a currency pair
func (e *Binance) GetOrderbook(pair string, limit int64) (*orderbook.Book, error) {
	sym, err := e.Pair(pair)
	if err != nil {
		return nil, fmt.Errorf("%s not found", pair)
	}
	resp := OrderBookData{}

	book, _ := orderbook.GetBook(pair)

	if limit == 0 || limit > 1000 {
		limit = 1000
	}

	params := url.Values{}
	params.Set("symbol", sym.Name)
	params.Set("limit", strconv.FormatInt(limit, 10))
	url := fmt.Sprintf("%s%s?%s", apiURL, depth, params.Encode())

	err = e.SendHTTPRequest("GET", url, false, &resp)
	if err != nil {
		return book, err
	}

	for _, asks := range resp.Asks {
		var p float64
		var a float64
		for i, v := range asks.([]interface{}) {
			switch i {
			case 0:
				p, _ = strconv.ParseFloat(v.(string), 64)
			case 1:
				a, _ = strconv.ParseFloat(v.(string), 64)
			}
			book.Add(p, a, false)
		}
	}
	for _, bids := range resp.Bids {
		var p float64
		var a float64
		for i, v := range bids.([]interface{}) {
			switch i {
			case 0:
				p, _ = strconv.ParseFloat(v.(string), 64)
			case 1:
				a, err = strconv.ParseFloat(v.(string), 64)
			}
			book.Add(p, a, true)
		}
	}

	return book, nil
}

// CancelOrder cancel an Order
func (e *Binance) CancelOrder(symbol string, id int64) (CancelOrderResponse, error) {
	resp := CancelOrderResponse{}

	params := url.Values{}
	params.Set("symbol", strings.ToUpper(symbol))
	params.Set("orderId", strconv.FormatInt(id, 10))

	url := fmt.Sprintf("%s%s?%s", apiURL, newOrder, params.Encode())
	err := e.SendHTTPRequest("DELETE", url, true, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Code != 0 {
		return resp, fmt.Errorf("%v %s", resp.Code, resp.Msg)
	}
	return resp, nil
}

// CheckOrder checks orderstatus
func (e *Binance) CheckOrder(symbol string, id int64) (OrderStatus, error) {
	resp := OrderStatus{}

	params := url.Values{}
	params.Set("symbol", strings.ToUpper(symbol))
	params.Set("orderId", strconv.FormatInt(id, 10))

	url := fmt.Sprintf("%s%s?%s", apiURL, newOrder, params.Encode())
	return resp, e.SendHTTPRequest("GET", url, true, &resp)
}

// NewOrder sends a new order to Binance
func (e *Binance) NewOrder(o NewOrderRequest) (NewOrderResponse, error) {
	resp := NewOrderResponse{}

	params := url.Values{}
	params.Set("symbol", o.Symbol)
	params.Set("side", o.Side)
	params.Set("type", o.TradeType)
	params.Set("quantity", strconv.FormatFloat(o.Quantity, 'f', -1, 64))
	if o.TradeType != "MARKET" {
		params.Set("timeInForce", o.TimeInForce)
		params.Set("price", strconv.FormatFloat(o.Price, 'f', -1, 64))
	}
	if o.StopPrice != 0.0 {
		params.Set("stopPrice", strconv.FormatFloat(o.StopPrice, 'f', -1, 64))
	}

	url := fmt.Sprintf("%s%s?%s", apiURL, newOrder, params.Encode())
	err := e.SendHTTPRequest("POST", url, true, &resp)
	if err != nil {
		return resp, err
	}
	if resp.Code != 0 {
		return resp, fmt.Errorf("%v %s", resp.Code, resp.Msg)
	}
	return resp, nil
}

// Wrappers to Exchange Interface -------------------------------------------------------------------------------------------------------------

// SendLimit Wrapper returns exchange pairs orderbook. Downloads if dont have any
func (e *Binance) SendLimit(pair, side string, amount, price float64) error {
	sym, err := e.Pair(pair)
	if err != nil {
		return fmt.Errorf("%s not found", pair)
	}
	i := getInfoIndex(sym.Name)
	if i < 0 {
		return fmt.Errorf("%s info not found", sym.Name)
	}

	resp, err := e.NewOrder(NewOrderRequest{
		Symbol:      sym.Name,
		Side:        strings.ToUpper(side),
		TradeType:   "LIMIT",
		Quantity:    utils.RoundPlus(amount, utils.CountDecimal(info.Symbols[i].Filters[2].StepSize)),
		Price:       utils.RoundPlus(price, utils.CountDecimal(info.Symbols[i].Filters[0].TickSize)),
		TimeInForce: "GTC",
	})
	if err != nil {
		return err
	}
	orders.Add(resp.OrderID, resp.Symbol, resp.Side, resp.OrigQty, resp.Price)
	return nil
}

// SendMarket Wrapper returns marketorder
func (e *Binance) SendMarket(pair, side string, amount float64) error {
	sym, err := e.Pair(pair)
	if err != nil {
		return fmt.Errorf("%s not found", pair)
	}
	i := getInfoIndex(sym.Name)
	if i < 0 {
		return fmt.Errorf("%s info not found", sym.Name)
	}

	_, err = e.NewOrder(NewOrderRequest{
		Symbol:    sym.Name,
		Side:      strings.ToUpper(side),
		TradeType: "MARKET",
		Quantity:  utils.RoundPlus(amount, utils.CountDecimal(info.Symbols[i].Filters[2].StepSize)),
	})
	if err != nil {
		return err
	}
	return nil
}

// SendCancel Wrapper canceles a order and removes from memory
func (e *Binance) SendCancel(pair string, id int64) error {
	sym, err := e.Pair(pair)
	if err != nil {
		return fmt.Errorf("%s not found", pair)
	}
	// _, err = e.CancelOrder(sym.Name, id)
	// if err != nil {
	// 	return err
	// }
	// orders.Delete(e.Name, id)
	// return nil

	_, err = e.CancelOrder(sym.Name, id)
	if err == nil || strings.Contains(fmt.Sprintf("%s", err), "-2011") || strings.Contains(fmt.Sprintf("%s", err), "-2013") {
		orders.Delete(id)
	}
	return err
}

// StreamBookDepth subscribes to symbols orderbooks and stream the top level depths
func (e *Binance) StreamBookDepth(pair string, doneC <-chan bool, notifyC chan<- string) error {
	sym, err := e.Pair(pair)
	if err != nil {
		return fmt.Errorf("%s not found\n", pair)
	}

	url := fmt.Sprintf(wsURL+"/ws/%s@depth20@100ms", strings.ToLower(sym.Name))
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial: %v\n", err)
	}

	defer func() {
		orderbook.Delete(pair)
		ws.Close()
	}()

	book, _ := orderbook.GetBook(sym.Name)

	for {
		select {
		case <-doneC:
			break
		default:
		}

		_, b, err := ws.ReadMessage()
		if err != nil {
			log.Printf("reconnecting %s due to: %v\n", sym.Name, err.Error())
			orderbook.Delete(sym.Name)
			go e.StreamBookDepth(sym.Name, doneC, notifyC)
			break
		}

		var resp DepthResponse
		if err := json.Unmarshal(b, &resp); err != nil {
			log.Println(err.Error())
			continue
		}

		book.Reset()
		for _, v := range resp.Asks {
			p, _ := strconv.ParseFloat(v[0].(string), 64)
			a, _ := strconv.ParseFloat(v[1].(string), 64)
			book.Asks.Add(p, a)
			// book.Add(p, a, false)
		}
		for _, v := range resp.Bids {
			p, _ := strconv.ParseFloat(v[0].(string), 64)
			a, _ := strconv.ParseFloat(v[1].(string), 64)
			book.Bids.Add(p, a)
			// book.Add(p, a, true)
		}
		if e.Debug {
			fmt.Printf("%-12s %-12d %-12d\n", sym.Name, len(book.Asks), len(book.Bids))
		}
		notifyC <- sym.Name
	}

	return nil
}

// SteamBookDeff stream only the new entry diffs made to orderbook
func (e *Binance) StreamBookDiff(pair string, done <-chan bool, notifyCh chan<- string) error {
	sym, err := e.Pair(pair)
	if err != nil {
		return fmt.Errorf("%s not found\n", pair)
	}

	url := fmt.Sprintf(wsURL+"/%s@depth", strings.ToLower(sym.Name))
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial: %v\n", err)
	}
	log.Printf("subscribed to %s %s orderbook\n", e.Name, sym.Name)

	go func() {
		defer func() {
			orderbook.Delete(pair)
			ws.Close()
		}()

		for {
			select {
			case <-done:
				break
			default:
			}

			_, b, err := ws.ReadMessage()
			if err != nil {
				log.Printf("ws error %s: %v\n", pair, err.Error())
				break
			}
			resp := struct {
				Type     string          `json:"e"`
				Time     float64         `json:"E"`
				Symbol   string          `json:"s"`
				UpdateID int             `json:"u"`
				Bids     [][]interface{} `json:"b"`
				Asks     [][]interface{} `json:"a"`
			}{}

			if err := json.Unmarshal(b, &resp); err != nil {
				log.Println(err.Error())
				continue
			}

			book, _ := orderbook.GetBook(resp.Symbol)
			// book.LastUpdated = time.Now()
			for _, v := range resp.Asks {
				p, _ := strconv.ParseFloat(v[0].(string), 64)
				a, _ := strconv.ParseFloat(v[1].(string), 64)
				// book.Asks.Add(p, a)
				book.Add(p, a, false)
			}
			for _, v := range resp.Bids {
				p, _ := strconv.ParseFloat(v[0].(string), 64)
				a, _ := strconv.ParseFloat(v[1].(string), 64)
				// book.Bids.Add(p, a)
				book.Add(p, a, true)
			}
			notifyCh <- resp.Symbol
		}
	}()

	return nil
}

// GetAllTickers returns map of all symbol prices
func (e *Binance) GetAllTickers() (map[string]float64, error) {
	resp, err := e.TickerAll()
	if err != nil {
		return nil, err
	}
	m := make(map[string]float64)
	for _, v := range resp {
		m[v.Symbol] = v.Price
	}
	return m, nil
}

// GetTicker returns price for specifik symbol
func (e *Binance) GetTicker(pair string) (float64, error) {
	sym, err := e.Pair(pair)
	if err != nil {
		return 0, fmt.Errorf("%s not found", pair)
	}

	resp, err := e.Ticker(sym.Name)
	if err != nil {
		return 0, err
	}
	return resp.Price, nil
}

// OrderStatus Wrapper checks if order exist
func (e *Binance) OrderStatus(id int64) (string, error) {
	o := orders.Orders[id]
	if o.Pair == "" {
		return "", fmt.Errorf("could not find order %d", id)
	}
	resp, err := e.CheckOrder(o.Pair, o.ID)
	if err != nil {
		return "", err
	}
	if resp.Status != "" {
		if resp.Status == "FILLED" || resp.Status == "CANCELED" || resp.Status == "REJECTED" || resp.Status == "EXPIRED" {
			orders.Delete(id)
		}
	}
	return resp.Status, nil
}

// OrderFills Wrapper checks if order exist
func (e *Binance) OrderFills(id int64) (float64, error) {
	o := orders.Orders[id]
	if o.Pair == "" {
		return 0, fmt.Errorf("could not find order %d", id)
	}
	resp, err := e.CheckOrder(o.Pair, o.ID)
	if err != nil {
		return 0, err
	}
	if resp.Status != "" {
		if resp.Status == "FILLED" || resp.Status == "CANCELED" || resp.Status == "REJECTED" || resp.Status == "EXPIRED" {
			orders.Delete(id)
		}
	}
	return resp.ExecutedQty, nil
}

// UpdateBalance Wrapper updates and returns exchange balances
func (e *Binance) UpdateBalance() error {
	tmp, err := e.AccountBalance()
	if err != nil {
		return err
	}

	for _, v := range tmp.Balances {
		if v.Free+v.Locked > 0 {
			asset := new(balance.Balance)
			asset.Asset = v.Asset
			asset.Free = v.Free
			asset.Locked = v.Locked
			asset.Update(strings.ToUpper(v.Asset))
		}
	}

	return nil
}

// UpdatePairs Wrapper
func (e *Binance) UpdatePairs() error {
	return e.SetPairs()
}

// GetKlines new data from Binance exchange
func (e *Binance) GetKlines(symbol, timeframe string, limit int) (history.Bars, error) {
	path := fmt.Sprintf(
		"https://api.binance.com/api/v1/klines?symbol=%s&interval=%s&limit=%v",
		strings.ToUpper(symbol), strings.ToLower(timeframe), limit)

	req, _ := http.NewRequest("GET", path, nil)
	req.Header.Add("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)

	tmp := [][]interface{}{}
	if err = json.Unmarshal(data, &tmp); err != nil {
		return nil, err
	}

	var bars history.Bars
	// convert into history.Bars
	for i, v := range tmp {

		bar := history.Bar{}
		bar.Time = time.Unix(int64(v[0].(float64))/1000, 0) // .UTC()
		bar.Open, err = strconv.ParseFloat(v[1].(string), 64)
		if err != nil {
			log.Printf("error bars[%d].Open\n", i)
		}
		bar.High, err = strconv.ParseFloat(v[2].(string), 64)
		if err != nil {
			log.Printf("error bars[%d].High\n", i)
		}
		bar.Low, err = strconv.ParseFloat(v[3].(string), 64)
		if err != nil {
			log.Printf("error bars[%d].Low\n", i)
		}
		bar.Close, err = strconv.ParseFloat(v[4].(string), 64)
		if err != nil {
			log.Printf("error bars[%d].Close\n", i)
		}
		bar.Volume, err = strconv.ParseFloat(v[5].(string), 64)
		if err != nil {
			log.Printf("error bars[%d].Volume\n", i)
		}

		// insert
		bars = append(history.Bars{bar}, bars...)
	}

	return bars, nil
}
