package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"googlemaps.github.io/maps"
)

const (
	placeDetailQPS      = 5    // limit PlaceDetails calls to avoid OVER_QUERY_LIMIT
	tileSizeMetres      = 500  // search radius per tile
	boundsBufferPercent = 0.15 // bounds expansion percentage
)

// validatePlacesSearchPrompt validates the search prompt format
func validatePlacesSearchPrompt(searchPrompt string) error {
	if searchPrompt == "" {
		return nil // not using search
	}

	if !strings.Contains(searchPrompt, " in ") {
		return fmt.Errorf("search prompt must be in format: \"[Business Type] in [Location]\"")
	}

	parts := strings.Split(searchPrompt, " in ")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("search prompt must contain both business type and location")
	}

	return nil
}

// searchURLsFromGooglePlaces queries Google Places for businesses matching
// provided keyword in specified location and extracts company URLs
// (uses tile-based grid approach to circumvent Places API limits)
func searchURLsFromGooglePlaces(ctx context.Context, searchPrompt string) ([]string, error) {
	if searchPrompt == "" {
		return nil, nil
	}

	keyword, location, _ := strings.Cut(searchPrompt, " in ")

	apiKey := os.Getenv("MAPS_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("failed to create maps client: Maps API key is required")
	}

	client, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create maps client: %w", err)
	}

	// geocode location to get bounding box
	bounds, err := geocodeBounds(ctx, client, location)
	if err != nil {
		return nil, err
	}

	// expand bounds to include outskirts
	expandedBounds := expandBounds(bounds, boundsBufferPercent)

	// generate tile centres
	tileCentres := generateTiles(expandedBounds, tileSizeMetres)

	urls := []string{}
	results := map[string]string{} // PlaceID -> Website

	ticker := time.NewTicker(time.Second / placeDetailQPS)
	defer ticker.Stop()

	for _, centre := range tileCentres {
		// get nearby places
		places, err := searchNearbyPlaces(ctx, client, keyword, centre.Lat, centre.Lng, tileSizeMetres)
		if err != nil {
			return nil, err
		}

		// get place details (needed for website data)
		for _, p := range places {
			// avoid duplicate PlaceDetails calls
			if _, exists := results[p.PlaceID]; exists {
				continue
			}

			<-ticker.C // throttle PlaceDetails

			// make a place details query
			details, err := client.PlaceDetails(ctx, &maps.PlaceDetailsRequest{
				PlaceID: p.PlaceID,
			})
			if err != nil {
				fmt.Printf("⚠️ failed place details for %s (ID: %s): %v\n", p.Name, p.PlaceID, err)
				continue
			}

			if details.Website == "" {
				continue
			}

			results[p.PlaceID] = details.Website
			urls = append(urls, details.Website)
		}
	}

	return urls, nil
}

// geocodeBounds gets the viewport bounds for a place name
func geocodeBounds(ctx context.Context, client *maps.Client, location string) (maps.LatLngBounds, error) {
	res, err := client.Geocode(ctx, &maps.GeocodingRequest{Address: location})
	if err != nil {
		return maps.LatLngBounds{}, fmt.Errorf("failed to geocode %s: %w", location, err)
	}

	if len(res) == 0 {
		return maps.LatLngBounds{}, fmt.Errorf("no geocode results for %s", location)
	}

	return res[0].Geometry.Bounds, nil
}

// expandBounds adds a buffer around the original bounds
func expandBounds(bounds maps.LatLngBounds, bufferPercent float64) maps.LatLngBounds {
	latRange := bounds.NorthEast.Lat - bounds.SouthWest.Lat
	lngRange := bounds.NorthEast.Lng - bounds.SouthWest.Lng

	latBuffer := latRange * bufferPercent
	lngBuffer := lngRange * bufferPercent

	return maps.LatLngBounds{
		NorthEast: maps.LatLng{
			Lat: bounds.NorthEast.Lat + latBuffer,
			Lng: bounds.NorthEast.Lng + lngBuffer,
		},
		SouthWest: maps.LatLng{
			Lat: bounds.SouthWest.Lat - latBuffer,
			Lng: bounds.SouthWest.Lng - lngBuffer,
		},
	}
}

// generateTiles splits bounds into tile centres for searches
func generateTiles(bounds maps.LatLngBounds, tileSize float64) []maps.LatLng {
	latStep := metresToLat(tileSize)
	lngStep := metresToLng(tileSize, (bounds.NorthEast.Lat+bounds.SouthWest.Lat)/2)

	var tiles []maps.LatLng
	for lat := bounds.SouthWest.Lat; lat <= bounds.NorthEast.Lat; lat += latStep {
		for lng := bounds.SouthWest.Lng; lng <= bounds.NorthEast.Lng; lng += lngStep {
			tiles = append(tiles, maps.LatLng{Lat: lat, Lng: lng})
		}
	}

	return tiles
}

// metresToLat converts metres to latitude degrees
func metresToLat(m float64) float64 {
	return m / 111320.0
}

// metresToLng converts metres to longitude degrees at a given latitude
func metresToLng(m, lat float64) float64 {
	return m / (111320.0 * math.Cos(lat*math.Pi/180))
}

// searchNearbyPlaces fetches up to 60 results for a given lat/lng,
// filtered by keyword
func searchNearbyPlaces(
	ctx context.Context,
	client *maps.Client,
	keyword string,
	lat, lng, radiusMetres float64,
) ([]maps.PlacesSearchResult, error) {
	allPlaces := []maps.PlacesSearchResult{}

	req := &maps.NearbySearchRequest{
		Location: &maps.LatLng{Lat: lat, Lng: lng},
		Radius:   uint(radiusMetres),
		Keyword:  keyword,
	}

	for {
		res, err := client.NearbySearch(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed nearby search for %v: %w", req.Location, err)
		}

		allPlaces = append(allPlaces, res.Results...)

		if res.NextPageToken == "" {
			break
		}

		req.PageToken = res.NextPageToken

		time.Sleep(2 * time.Second) // required delay before next page
	}

	return allPlaces, nil
}
