package geo

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/oschwald/geoip2-golang/v2"
)

type maxmindIPLocatorV2 struct {
	db *geoip2.Reader
}

func NewMaxmindIPLocatorV2(dbFile string) (IPLocator, error) {
	// try opening the provided file first, then fall back to common filename(s)
	db, err1 := tryOpenGeoDB(dbFile, "GeoLite2-City.mmdb")
	if err1 == nil {
		return &maxmindIPLocatorV2{db: db}, nil
	}
	db, err2 := tryOpenEmbedGeoDB()
	if err2 == nil {
		return &maxmindIPLocatorV2{db: db}, nil
	}
	return nil, fmt.Errorf("try both read file and embed file failed, error: %w, %w", err1, err2)
}

// tryOpenGeoDB attempts to open the geoip2 database from a list of candidate
// paths in order. It returns the first successfully opened reader, or an
// aggregated error listing all attempts.
func tryOpenGeoDB(files ...string) (*geoip2.Reader, error) {
	var attempts []string
	var lastErr error
	for _, f := range files {
		if f == "" {
			continue
		}
		attempts = append(attempts, f)
		db, err := geoip2.Open(f)
		if err == nil {
			return db, nil
		}
		lastErr = err
	}

	// Build a clear error message that shows what paths were tried.
	if len(attempts) == 0 {
		return nil, fmt.Errorf("no geoip2 database path provided")
	}
	msg := "failed to open geoip2 database; attempted paths:"
	for _, p := range attempts {
		msg += " - " + p
	}
	if lastErr != nil {
		msg += " last error: " + lastErr.Error()
	}
	return nil, errors.New(msg)
}

func tryOpenEmbedGeoDB() (*geoip2.Reader, error) {
	dbBytes, err := geoFS.ReadFile("GeoLite2-City.mmdb")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded geoip2 database: %w", err)
	}
	db, err := geoip2.OpenBytes(dbBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create geoip2 reader from embedded data: %w", err)
	}
	return db, nil
}

func (m *maxmindIPLocatorV2) GetIPLocation(ipStr string) (*IPLocation, error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil, fmt.Errorf("parse ip %s failed, error: %w", ipStr, err)
	}
	record, err := m.db.City(ip)
	if err != nil {
		return nil, fmt.Errorf("failed to get city for ip %s: %w", ipStr, err)
	}

	var province, city string
	if len(record.Subdivisions) > 0 {
		province = record.Subdivisions[0].Names.English
	}
	if record.City.Names.English != "" {
		city = record.City.Names.English
	}

	return &IPLocation{
		Nation:   record.Country.Names.English,
		Province: province,
		City:     city,
	}, nil
}
