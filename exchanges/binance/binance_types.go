package binance

// ExchangeInfo holds the full exchange information type
type ExchangeInfo struct {
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	Timezone   string `json:"timezone"`
	Servertime int64  `json:"serverTime"`
	RateLimits []struct {
		RateLimitType string `json:"rateLimitType"`
		Interval      string `json:"interval"`
		Limit         int    `json:"limit"`
	} `json:"rateLimits"`
	ExchangeFilters interface{} `json:"exchangeFilters"`
	Symbols         []struct {
		Symbol             string   `json:"symbol"`
		Status             string   `json:"status"`
		BaseAsset          string   `json:"baseAsset"`
		BaseAssetPrecision int      `json:"baseAssetPrecision"`
		QuoteAsset         string   `json:"quoteAsset"`
		QuotePrecision     int      `json:"quotePrecision"`
		OrderTypes         []string `json:"orderTypes"`
		IcebergAllowed     bool     `json:"icebergAllowed"`
		Filters            []struct {
			FilterType  string  `json:"filterType"`
			MinPrice    float64 `json:"minPrice,string"`
			MaxPrice    float64 `json:"maxPrice,string"`
			TickSize    float64 `json:"tickSize,string"`
			MinQty      float64 `json:"minQty,string"`
			MaxQty      float64 `json:"maxQty,string"`
			StepSize    float64 `json:"stepSize,string"`
			MinNotional float64 `json:"minNotional,string"`
		} `json:"filters"`
	} `json:"symbols"`
}

// Account holds account result
type Account struct {
	MakerCommission  int64     `json:"makerCommission"`
	TakerCommission  int64     `json:"takerCommission"`
	BuyerCommission  int64     `json:"buyerCommission"`
	SellerCommission int64     `json:"sellerCommission"`
	CanTrade         bool      `json:"canTrade"`
	CanWithdraw      bool      `json:"canWithdraw"`
	CanDeposit       bool      `json:"canDeposit"`
	Balances         []Balance `json:"balances"`
}

// Balance returns account balances
type Balance struct {
	Asset  string  `json:"asset"`
	Free   float64 `json:"free,string"`
	Locked float64 `json:"locked,string"`
}

// OrderBookData is resp data from orderbook endpoint
type OrderBookData struct {
	Code         int           `json:"code"`
	Msg          string        `json:"msg"`
	LastUpdateID int64         `json:"lastUpdateId"`
	Bids         []interface{} `json:"bids"`
	Asks         []interface{} `json:"asks"`
}

// OrderBook actual structured data that can be used for orderbook
type OrderBook struct {
	Code int
	Msg  string
	Bids []struct {
		Price    float64
		Quantity float64
	}
	Asks []struct {
		Price    float64
		Quantity float64
	}
}

// NewOrderRequest request type
type NewOrderRequest struct {
	Symbol           string
	Side             string
	TradeType        string
	TimeInForce      string
	Quantity         float64
	Price            float64
	NewClientOrderID string
	StopPrice        float64
	IcebergQty       float64
	NewOrderRespType string
}

// NewOrderResponse is the return structured response from the exchange
type NewOrderResponse struct {
	Code            int     `json:"code"`
	Msg             string  `json:"msg"`
	Symbol          string  `json:"symbol"`
	OrderID         int64   `json:"orderId"`
	ClientOrderID   string  `json:"clientOrderId"`
	TransactionTime int64   `json:"transactTime"`
	Price           float64 `json:"price,string"`
	OrigQty         float64 `json:"origQty,string"`
	ExecutedQty     float64 `json:"executedQty,string"`
	Status          string  `json:"status"`
	TimeInForce     string  `json:"timeInForce"`
	Type            string  `json:"type"`
	Side            string  `json:"side"`
	Fills           []struct {
		Price           float64 `json:"price,string"`
		Qty             float64 `json:"qty,string"`
		Commission      float64 `json:"commission,string"`
		CommissionAsset string  `json:"commissionAsset"`
	} `json:"fills"`
}

// Result from: GET /api/v3/order
type OrderStatus struct {
	Symbol        string  `json:"symbol"`
	OrderId       int64   `json:"orderId"`
	ClientOrderId string  `json:"clientOrderId"`
	TransactTime  int64   `json:"transactTime"`
	Price         float64 `json:"price,string"`
	OrigQty       float64 `json:"origQty,string"`
	ExecutedQty   float64 `json:"executedQty,string"`
	Status        string  `json:"status"`
	TimeInForce   string  `json:"timeInForce"`
	Type          string  `json:"type"`
	Side          string  `json:"side"`
	StopPrice     float64 `json:"stopPrice,string"`
	IcebergQty    float64 `json:"icebergQty,string"`
	Time          int64   `json:"time"`
}

// SymbolPrice holds basic symbol price
type SymbolPrice struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price,string"`
}

// Result from: GET /api/v1/allBookTickers
type BookTicker struct {
	Symbol      string  `json:"symbol"`
	BidPrice    float64 `json:"bidPrice,string"`
	BidQuantity float64 `json:"bidQty,string"`
	AskPrice    float64 `json:"askPrice,string"`
	AskQuantity float64 `json:"askQty,string"`
}

// CancelOrderResponse
type CancelOrderResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	Symbol            string `json:"symbol"`
	OrigClientOrderId string `json:"origClientOrderId"`
	OrderId           int64  `json:"orderId"`
	ClientOrderId     string `json:"clientOrderId"`
}

type OrderHistory struct {
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Id              int64  `json:"id"`
	IsBestMatch     bool   `json:"isBestMatch"`
	IsBuyer         bool   `json:"isBuyer"`
	IsMaker         bool   `json:"isMaker"`
	OrderId         int64  `json:"orderId"`
	OrderListId     int64  `json:"orderListId"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	Symbol          string `json:"symbol"`
	Time            int64  `json:"time"`
}

// WsDepthEvent - ws orderbook data
type DepthEvent struct {
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

type MyTrades struct {
	Symbol          string `json:"symbol"`
	Id              int64  `json:"id"`
	OrderID         int64  `json:"orderId"`
	OrderListID     int64  `json:"orderListId"` //Unless OCO, the value will always be -1
	Price           string `json:"price"`
	Quantity        string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	Commmission     string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Time            int64  `json:"time"`
	IsBuyer         bool   `json:"isBuyer"`
	IsMaker         bool   `json:"isMaker"`
	IsBestMatch     bool   `json:"isBestMatch"`
}
