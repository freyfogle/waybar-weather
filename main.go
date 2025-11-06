package main

import (
	"context"
	"fmt"
	"os"
	"time"

	nominatim "github.com/doppiogancio/go-nominatim"
	"github.com/hectormalot/omgo"
	"github.com/maltegrosse/go-geoclue2"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	client, err := omgo.NewClient()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create client: %s\n", err)
		os.Exit(1)
	}
	lat, long, err := getLocation()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to get latitude/longitude: %s\n", err)
		os.Exit(1)
	}
	address, err := nominatim.ReverseGeocode(lat, long, "english")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to reverse geocode: %s\n", err)
		os.Exit(1)
	}

	location, err := omgo.NewLocation(lat, long)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to set location: %s\n", err)
		os.Exit(1)
	}
	current, err := client.CurrentWeather(ctx, location, &omgo.Options{
		Timezone: "Europe/Berlin",
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to get current weather: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Data at %s (lat: %f, lon: %f): %+v\n", address.City, lat, long, current)
}

func getLocation() (float64, float64, error) {
	gcm, err := geoclue2.NewGeoclueManager()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create geoclue manager: %w", err)
	}

	client, err := gcm.GetClient()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get client: %w", err)
	}
	if err = client.SetDesktopId("waybar-weather"); err != nil {
		return 0, 0, fmt.Errorf("failed to set desktop id: %w", err)
	}

	if err = client.SetRequestedAccuracyLevel(geoclue2.GClueAccuracyLevelStreet); err != nil {
		return 0, 0, fmt.Errorf("failed to set requested accuracy level: %w", err)
	}

	// Get RequestedAccuracyLevel
	/*
		level, err := client.GetRequestedAccuracyLevel()
		if err != nil {
			return 0,0, fmt.Errorf("failed to get requested accuracy level: %w", err)
		}

	*/

	if err = client.Start(); err != nil {
		return 0, 0, fmt.Errorf("failed to start client: %w", err)
	}

	// create new Instance of Geoclue Location
	location, err := client.GetLocation()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get location: %w", err)
	}

	latitude, err := location.GetLatitude()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get latitude: %w", err)
	}

	// get longitude
	longitude, err := location.GetLongitude()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get longitude: %w", err)
	}

	return latitude, longitude, nil
}
