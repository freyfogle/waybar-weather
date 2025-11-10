package geoip

import (
	"context"
	"fmt"
	"time"

	"app/internal/geobus"
	"app/internal/http"
)

const (
	APIEndpoint   = "https://reallyfreegeoip.org/json/"
	LookupTimeout = time.Second * 5
)

const (
	AccuracyCountry = 300000
	AccuracyRegion  = 100000
	AccuracyCity    = 15000
	AccuracyZip     = 3000
	AccuarcyUnknown = 1000000
)

type GeolocationGeoIPProvider struct {
	name   string
	result geobus.Result
	http   *http.Client
	period time.Duration
	ttl    time.Duration
}

type APIResult struct {
	IP          string  `json:"ip"`
	CountryCode string  `json:"country_code"`
	Country     string  `json:"country_name"`
	RegionCode  string  `json:"region_code,omitempty"`
	Region      string  `json:"region_name,omitempty"`
	City        string  `json:"city,omitempty"`
	ZipCode     string  `json:"zip_code,omitempty"`
	TimeZone    string  `json:"time_zone"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	MetroCode   int     `json:"metro_code"`
}

func NewGeolocationGeoIPProvider(http *http.Client) *GeolocationGeoIPProvider {
	return &GeolocationGeoIPProvider{
		name:   "geoip",
		http:   http,
		period: 30 * time.Minute,
		ttl:    60 * time.Minute,
	}
}

func (p *GeolocationGeoIPProvider) Name() string {
	return p.name
}

// LookupStream continuously streams geolocation results from a file, emitting updates when data changes
// or context ends.
func (p *GeolocationGeoIPProvider) LookupStream(ctx context.Context, key string) <-chan geobus.Result {
	out := make(chan geobus.Result)
	go func() {
		defer close(out)
		state := geobus.GeolocationState{}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			lat, lon, alt, acc, con, err := p.locate(ctx)
			if err != nil {
				time.Sleep(p.period)
				continue
			}

			// Only emit if values changed or it's the first read
			if state.HasChanged(lat, lon, alt, acc) {
				state.Update(lat, lon, alt, acc)
				r := p.createResult(key, lat, lon, alt, acc, con)

				select {
				case <-ctx.Done():
					return
				case out <- r:
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(p.period):
			}
		}
	}()
	return out
}

// createResult composes and returns a Result using provided geolocation data and metadata.
func (p *GeolocationGeoIPProvider) createResult(key string, lat, lon, alt, acc, con float64) geobus.Result {
	return geobus.Result{
		Key:            key,
		Lat:            lat,
		Lon:            lon,
		Alt:            alt,
		AccuracyMeters: acc,
		Confidence:     con,
		Source:         p.name,
		At:             time.Now(),
		TTL:            p.ttl,
	}
}

func (p *GeolocationGeoIPProvider) locate(ctx context.Context) (lat, lon, alt, acc, con float64, err error) {
	ctxHttp, cancelHttp := context.WithTimeout(ctx, LookupTimeout)
	defer cancelHttp()

	result := new(APIResult)
	if _, err = p.http.Get(ctxHttp, APIEndpoint, result, nil); err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to get geolocation data from API: %w", err)
	}

	acc = AccuarcyUnknown
	con = 0.1
	if result.ZipCode != "" {
		acc = AccuracyZip
		con = 0.85
	}
	if result.City != "" {
		acc = AccuracyCity
		con = 0.7
	}
	if result.RegionCode != "" {
		acc = AccuracyRegion
		con = 0.5
	}
	if result.CountryCode != "" {
		acc = AccuracyCountry
		con = 0.3
	}

	return result.Latitude, result.Longitude, 0, acc, con, nil
}
