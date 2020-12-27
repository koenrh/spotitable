package main

import (
	"log"

	"github.com/fabioberger/airtable-go"
)

type Airtable struct {
	apiKey string
	baseID string
	client *airtable.Client
}

func NewAirtable(apiKey string, baseID string) *Airtable {
	airtableClient, err := airtable.New(apiKey, baseID)
	if err != nil {
		log.Fatal(err)
	}

	return &Airtable{
		client: airtableClient,
		apiKey: apiKey,
		baseID: baseID,
	}
}

func (c *Airtable) GetTrackIDs(table string, formula string) ([]string, error) {
	type Fields struct {
		Year      int
		Like      bool
		Love      bool
		BPM       int
		SpotifyID string `json:"Spotify ID"`
	}

	type trackRecord struct {
		ID     string `json:"id,omitempty"`
		Fields Fields `json:"fields,omitempty"`
	}

	listParams := airtable.ListParameters{
		Fields:          []string{"Year", "Like", "Love", "BPM", "Spotify ID"},
		FilterByFormula: formula,
	}
	trackRecords := []trackRecord{}

	if err := c.client.ListRecords(table, &trackRecords, listParams); err != nil {
		return nil, err
	}

	trackIDs := []string{}
	for _, record := range trackRecords {
		trackIDs = append(trackIDs, record.Fields.SpotifyID)
	}

	return trackIDs, nil
}
