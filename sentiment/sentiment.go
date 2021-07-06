package sentiment

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type SentimentData struct {
	Value     string `json:"value"`
	ValueName string `json:"value_classification"`
	Timestamp string `json:"timestamp"`
	TimeUntil string `json:"time_until_update"`
}
type sentimentBlob struct {
	Name     string        `json:"name"`
	Data     sentimentList `json:"data"`
	Metadata interface{}   `json:"metadata"`
}

type sentimentList []SentimentData

func GetSentimentIndex() (SentimentData, error) {
	resp, err := http.Get("https://api.alternative.me/fng/")
	if err != nil || resp.StatusCode != 200 {
		return SentimentData{}, err
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return SentimentData{}, err
	}
	resp.Body.Close()

	sentiment := sentimentBlob{}
	json.Unmarshal(data, &sentiment)
	return sentiment.Data[0], nil

}
