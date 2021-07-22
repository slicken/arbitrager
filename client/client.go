package client

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// Requester struct for the request client
type Requester struct {
	Name       string
	HTTPClient *http.Client
	Debug      bool
}

// DefaultHTTPTimeout holds default timeout
var DefaultHTTPTimeout = time.Second * 15

// NewHTTPClient sets new http client
func NewHTTPClient(t time.Duration) *http.Client {
	return &http.Client{Timeout: t}
}

// NewRequester returns a new Requester
func NewRequester(name string, httpRequester *http.Client) *Requester {
	r := &Requester{HTTPClient: httpRequester, Name: name, Debug: false}
	return r
}

// Do sends and processes the request
func (r *Requester) Do(req *http.Request, method, path string, auth bool, result interface{}) error {
	if r == nil || r.Name == "" {
		return errors.New("not initalized")
	}
	if r.Debug {
		log.Printf("%s request: %s", r.Name, path)
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if r.Debug {
		log.Printf("%s response: %s", r.Name, string(content[:]))
	}
	if result != nil {
		return json.Unmarshal(content, result)
	}

	return nil
}

type RateLimit struct {
	value int
	rates []rate
}

type rate struct {
	waight int
	expiry time.Time
}

func NewRateLimit(value int) *RateLimit {
	return &RateLimit{value: value}
}

func (r *RateLimit) Add(waight int, expiry time.Time) {
	r.rates = append(r.rates, rate{waight: waight, expiry: expiry})
}

func (r *RateLimit) Get() int {
	var n = make([]rate, 0)

	rate := r.value
	for _, v := range r.rates {
		if time.Now().After(v.expiry) {
			continue
		}
		n = append(n, v)
		rate -= v.waight
	}

	r.rates = n
	return rate
}
