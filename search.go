package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"googlemaps.github.io/maps"
)

const (
	apiKey           = "AIzaSyBPFeYrbJBhQ0Zs35bIER3lmW_j-FKO3ak"
	placeDetailQPS   = 5    // limit PlaceDetails calls to avoid OVER_QUERY_LIMIT
	radiusSizeMetres = 5000 // search radius in metres
)

// searchURLsFromGooglePlaces queries Google Places for businesses matching
// provided keyword in specified location and extracts company URLs
func searchURLsFromGooglePlaces(ctx context.Context, searchPrompt string) ([]string, error) {
	keyword, location, split := strings.Cut(searchPrompt, " in ")
	if !split {
		return nil, fmt.Errorf("search prompt must be in the following format: \"[Business Type] in [Location]\"")
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

	// use the midpoint of the bounds as the search centre
	centre := maps.LatLng{
		Lat: (bounds.NorthEast.Lat + bounds.SouthWest.Lat) / 2,
		Lng: (bounds.NorthEast.Lng + bounds.SouthWest.Lng) / 2,
	}

	// get nearby places
	nearbyPlaces, err := searchNearbyPlaces(ctx, client, keyword, centre.Lat, centre.Lng, radiusSizeMetres)
	if err != nil {
		return nil, err
	}

	checkedDomains := map[string]bool{}
	urls := []string{}
	results := map[string]string{} // PlaceID -> Website

	ticker := time.NewTicker(time.Second / placeDetailQPS)
	defer ticker.Stop()

	// get place details (needed for website data)
	for _, p := range nearbyPlaces {
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
			fmt.Printf("failed place details for %s (ID: %s): %v", p.Name, p.PlaceID, err)
			continue
		}

		if details.Website == "" {
			continue
		}

		scheme, domain, err := extractUrlParts(details.Website)
		if err != nil {
			fmt.Printf("failed URL parsing for %s (ID: %s): %v", p.Name, p.PlaceID, err)
			continue
		}

		if checkedDomains[domain] {
			continue
		}

		urls = append(urls, scheme+"://"+domain+"/")
		checkedDomains[domain] = true
		results[p.PlaceID] = details.Website
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
