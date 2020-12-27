package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	epoch       = 1970
	decadeYears = 10

	// Prefix playlists to avoid the chance of overwriting any user-managed playlist.
	playlistPrefix = "st"
)

func main() {
	var airtableBaseID, airtableTable string

	flag.StringVar(&airtableBaseID, "base", "", "Airtable base ID.")
	flag.StringVar(&airtableTable, "table", "", "Airtable table.")

	flag.Parse()

	if airtableBaseID == "" || airtableTable == "" {
		fmt.Fprintf(os.Stderr, "Usage of %s [options]:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	for _, v := range []string{
		"SPOTIFY_CLIENT_ID",
		"SPOTIFY_CLIENT_SECRET",
		"AIRTABLE_API_KEY",
	} {
		_, ok := os.LookupEnv(v)

		if !ok {
			fmt.Printf("required environment variable %s not set\n", v)

			return
		}
	}

	spotifyClient := NewSpotify(os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"))
	spotifyClient.StartAuthentication()

	select {
	case err := <-spotifyClient.errors:
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	case result := <-spotifyClient.results:
		fmt.Printf("login spotify:user:%s\n", result)
	}

	airtableClient := NewAirtable(os.Getenv("AIRTABLE_API_KEY"), airtableBaseID)

	// create year-based playlists
	now := time.Now()
	currentYear := now.Year()

	for i := epoch; i <= currentYear; i++ {
		trackIDs, err := airtableClient.GetTrackIDs(airtableTable, fmt.Sprintf("{Year} = %d", i))
		if err != nil {
			log.Fatal(err)
		}

		playlistName := fmt.Sprintf("%s-year-%d", playlistPrefix, i)
		spotifyClient.AddTracksToNamedPlaylist(playlistName, trackIDs)
	}

	// create decade-based playlists
	for i := epoch; i < (currentYear + decadeYears); i += decadeYears {
		trackIDs, err := airtableClient.GetTrackIDs(airtableTable, fmt.Sprintf("AND({Year} >= %d, {Year} < %d)", i, i+decadeYears))
		if err != nil {
			log.Fatal(err)
		}

		playlistName := fmt.Sprintf("%s-decade-%ds", playlistPrefix, i)
		spotifyClient.AddTracksToNamedPlaylist(playlistName, trackIDs)
	}

	// create liked playlist
	likedTrackIDs, err := airtableClient.GetTrackIDs(airtableTable, "{Like} = 1")
	if err != nil {
		log.Fatal(err)
	}

	spotifyClient.AddTracksToNamedPlaylist(fmt.Sprintf("%s-liked", playlistPrefix), likedTrackIDs)

	// create loved tracks
	lovedTrackIDs, err := airtableClient.GetTrackIDs(airtableTable, "{Love} = 1")
	if err != nil {
		log.Fatal(err)
	}

	spotifyClient.AddTracksToNamedPlaylist(fmt.Sprintf("%s-loved", playlistPrefix), lovedTrackIDs)
}
