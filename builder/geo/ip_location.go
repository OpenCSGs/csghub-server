package geo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type IPLocator interface {
	GetIPLocation(ip string) (*IPLocation, error)
}

type IPLocation struct {
	Nation   string `json:"nation"`
	Province string `json:"province"`
	City     string `json:"city"`
	District string `json:"district"`
	ADCode   string `json:"adcode"`
}

type gaodeIPLocator struct {
	host string
	api  string

	Key string
}

func NewGaodeIPLocator(key string) IPLocator {
	return &gaodeIPLocator{
		host: "https://restapi.amap.com",
		api:  "/v3/ip",
		Key:  key,
	}
}

func (g *gaodeIPLocator) GetIPLocation(ip string) (*IPLocation, error) {
	//see gaode api doc: https://lbs.amap.com/api/webservice/guide/api/ipconfig/
	url := fmt.Sprintf("%s%s?ip=%s&key=%s", g.host, g.api, ip, g.Key)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call gaode ip location api: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read gaode ip location api response: %w", err)
	}
	var gaodeIPLocationResponse gaodeIPLocationResponse
	err = json.Unmarshal(body, &gaodeIPLocationResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal gaode ip location api response: %w", err)
	}
	if gaodeIPLocationResponse.Status != "1" {
		return nil, fmt.Errorf("gaode ip location api status is not 1, message: %s", gaodeIPLocationResponse.Info)
	}
	return &IPLocation{
		Province: gaodeIPLocationResponse.Province,
		City:     gaodeIPLocationResponse.City,
		ADCode:   gaodeIPLocationResponse.ADCode,
	}, nil
}

type gaodeIPLocationResponse struct {
	Status   string `json:"status"`
	Info     string `json:"info"`
	InfoCode string `json:"infocode"`
	Province string `json:"province"`
	City     string `json:"city"`
	ADCode   string `json:"adcode"`
}
