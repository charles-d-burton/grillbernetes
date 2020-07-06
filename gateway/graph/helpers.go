package graph

import (
	"bytes"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

func makeReq(url string, data []byte) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data)) //This is inefficient, should change to pool of handlers with re-usable buffers
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

var (
	json    = jsoniter.ConfigCompatibleWithStandardLibrary
	log     = logrus.New()
	authURL = "https://auth.home.rsmachiner.com"
)
