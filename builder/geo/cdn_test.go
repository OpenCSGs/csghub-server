package geo_test

import (
	"fmt"
	"testing"

	"opencsg.com/csghub-server/builder/geo"
	"opencsg.com/csghub-server/common/config"
)

func TestCDNUrl(t *testing.T) {
	config := config.Config{}
	config.CityToCdnDomain = map[string]string{
		"Shanghai": "cdn-lfs-sh-1.opencsg.com",
	}
	geo.Config(&config)
	geo.SetIPLocator(&mockIPLocator{})
	cdnUrl, err := geo.CDNUrlString("219.142.137.156", "http://localhost:8080/api/v1/hello")
	if err != nil {
		t.Fatalf("failed to get cdn url, %v", err)
	}
	if cdnUrl != "http://cdn-lfs-sh-1.opencsg.com/api/v1/hello" {
		t.Errorf("cdn url is not correct, %s", cdnUrl)
	}
}
func TestCDNUrl_CantFindCdnDomain(t *testing.T) {
	config := config.Config{}
	config.CityToCdnDomain = map[string]string{
		"Shanghai": "cdn-lfs-sh-1.opencsg.com",
	}
	geo.Config(&config)
	geo.SetIPLocator(&errorIPLocator{})
	cdnUrl, err := geo.CDNUrlString("219.142.137.156", "http://localhost:8080/api/v1/hello")
	if err == nil {
		t.Fatalf("should get error when can get cdn domain")
	}
	if cdnUrl != "http://localhost:8080/api/v1/hello" {
		t.Errorf("cdn url is not correct, %s", cdnUrl)
	}
}

type mockIPLocator struct {
}

func (m *mockIPLocator) GetIPLocation(ip string) (*geo.IPLocation, error) {
	return &geo.IPLocation{
		City: "Shanghai",
	}, nil
}

type errorIPLocator struct {
}

func (m *errorIPLocator) GetIPLocation(ip string) (*geo.IPLocation, error) {
	return nil, fmt.Errorf("failed to get ip location")
}
