package geolocation_file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"app/internal/geobus"
)

// GeolocationFileProvider reads geolocation data from a file and emits updates via a stream.
// It periodically reads a specified file, parses its data, and updates geolocation results based on changes.
// Each result includes details about the location, accuracy, confidence, and timestamp of the data.
// Results are subject to a time-to-live (TTL) duration, ensuring outdated data is discarded.
type GeolocationFileProvider struct {
	name   string
	result geobus.Result
	path   string
	period time.Duration
	ttl    time.Duration
}

// NewGeolocationFileProvider initializes a GeolocationFileProvider with a file path and default update
// interval and TTL settings.
func NewGeolocationFileProvider(path string) *GeolocationFileProvider {
	return &GeolocationFileProvider{
		name:   "GeolocationFile",
		path:   path,
		period: 2 * time.Minute,
		ttl:    15 * time.Minute,
	}
}

// Name returns the name of the GeolocationFileProvider instance.
func (p *GeolocationFileProvider) Name() string {
	return p.name
}

// LookupStream continuously streams geolocation results from a file, emitting updates when data changes
// or context ends.
func (p *GeolocationFileProvider) LookupStream(ctx context.Context, key string) <-chan geobus.Result {
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

			lat, lon, alt, acc, err := p.readFile()
			if err != nil {
				// File missing or malformed — just retry later
				time.Sleep(p.period)
				continue
			}

			// Only emit if values changed or it's the first read
			if state.HasChanged(lat, lon, alt, acc) {
				state.Update(lat, lon, alt, acc)
				r := p.createResult(key, lat, lon, alt, acc)

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
func (p *GeolocationFileProvider) createResult(key string, lat, lon, alt, acc float64) geobus.Result {
	return geobus.Result{
		Key:            key,
		Lat:            lat,
		Lon:            lon,
		Alt:            alt,
		AccuracyMeters: acc,
		Confidence:     1.0,
		Source:         p.name,
		At:             time.Now(),
		TTL:            p.ttl,
	}
}

// readFile reads geolocation data from the file at the configured path.
// Returns latitude, longitude, altitude, accuracy, or an error if the file cannot be
// read or parsed correctly.
func (p *GeolocationFileProvider) readFile() (lat, lon, alt, acc float64, err error) {
	f, err := os.Open(p.path)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("open %s: %w", p.path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close geolocation file: %w", closeErr))
		}
	}()

	scanner := bufio.NewScanner(f)
	var values []float64
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		v, parseErr := strconv.ParseFloat(line, 64)
		if parseErr != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid number in %s: %w", p.path, parseErr)
		}
		values = append(values, v)
	}
	if len(values) < 4 {
		return 0, 0, 0, 0, errors.New("geolocation file missing required lines (need 4)")
	}
	return values[0], values[1], values[2], values[3], nil
}
