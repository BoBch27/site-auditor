package main

import (
	"context"
	"fmt"
	"time"

	"googlemaps.github.io/maps"
)

const apiKey = "AIzaSyBPFeYrbJBhQ0Zs35bIER3lmW_j-FKO3ak"

// searchURLsFromGooglePlaces queries Google Places for businesses matching
// provided keyword in specified location and extracts company URLs
func searchURLsFromGooglePlaces(ctx context.Context, searchPrompt string) ([]string, error) {
	client, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create maps client: %w", err)
	}

	allPlaces := []maps.PlacesSearchResult{}

	// make a text query
	textReq := &maps.TextSearchRequest{Query: searchPrompt}
	for {
		res, err := client.TextSearch(ctx, textReq)
		if err != nil {
			return nil, fmt.Errorf("failed text search for %s: %w", searchPrompt, err)
		}

		allPlaces = append(allPlaces, res.Results...)

		if res.NextPageToken == "" {
			break
		}

		textReq.PageToken = res.NextPageToken

		time.Sleep(2 * time.Second) // required delay before next page
	}

	checkedDomains := map[string]bool{}
	urls := []string{}

	for _, p := range allPlaces {
		// get place details (needed for website data)
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
	}

	return urls, nil
}
