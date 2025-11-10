package ichnaea

import (
	"context"
	"fmt"
	"time"

	"app/internal/geobus"
	"app/internal/http"
)

const (
	APIEndpoint   = "https://api.beacondb.net/v1/geolocate"
	LookupTimeout = time.Second * 5
)

type GeolocationICHNAEAProvider struct {
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

func NewGeolocationICHNAEAProvider(http *http.Client) *GeolocationICHNAEAProvider {
	return &GeolocationICHNAEAProvider{
		name:   "geoip",
		http:   http,
		period: 30 * time.Minute,
		ttl:    60 * time.Minute,
	}
}

func (p *GeolocationICHNAEAProvider) Name() string {
	return p.name
}

// LookupStream continuously streams geolocation results from a file, emitting updates when data changes
// or context ends.
func (p *GeolocationICHNAEAProvider) LookupStream(ctx context.Context, key string) <-chan geobus.Result {
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
func (p *GeolocationICHNAEAProvider) createResult(key string, lat, lon, alt, acc, con float64) geobus.Result {
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

func (p *GeolocationICHNAEAProvider) locate(ctx context.Context) (lat, lon, alt, acc, con float64, err error) {
	ctxHttp, cancelHttp := context.WithTimeout(ctx, LookupTimeout)
	defer cancelHttp()

	result := new(APIResult)
	if _, err = p.http.Get(ctxHttp, APIEndpoint, result, nil); err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to get geolocation data from API: %w", err)
	}

	return result.Latitude, result.Longitude, 0, acc, con, nil
}
