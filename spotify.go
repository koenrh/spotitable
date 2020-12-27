package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/pkg/browser"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

const (
	redirectURI = "http://127.0.0.1:8080/callback"

	// NOTE: Maximum number of tracks that can be specified in one request.
	// https://developer.spotify.com/documentation/web-api/reference/tracks/get-several-tracks/
	maxTracks = 50

	// NOTE: Maxmium number of playlists that can be specified in one request.
	// https://developer.spotify.com/documentation/web-api/reference/playlists/get-a-list-of-current-users-playlists/
	maxPlaylists = 50

	// NOTE: Maximum number of playlist tracks that can be specified in a one request.
	// https://developer.spotify.com/documentation/web-api/reference/playlists/get-playlists-tracks/
	// https://developer.spotify.com/documentation/web-api/reference/playlists/remove-tracks-playlist/
	// https://developer.spotify.com/documentation/web-api/reference/playlists/add-tracks-to-playlist/
	maxPlaylistTracks = 100

	numRandomBytes = 32
)

type SpotifyClient interface {
	GetTracks(...spotify.ID) ([]*spotify.FullTrack, error)
	GetPlaylistsForUserOpt(string, *spotify.Options) (*spotify.SimplePlaylistPage, error)
	GetPlaylistTracksOpt(spotify.ID, *spotify.Options, string) (*spotify.PlaylistTrackPage, error)
	CreatePlaylistForUser(string, string, string, bool) (*spotify.FullPlaylist, error)
	AddTracksToPlaylist(spotify.ID, ...spotify.ID) (string, error)
	RemoveTracksFromPlaylist(spotify.ID, ...spotify.ID) (string, error)
}

type Spotify struct {
	client        SpotifyClient
	authenticator spotify.Authenticator

	clientID      string
	clientSecret  string
	state         string
	codeVerifier  string
	codeChallenge string

	errors  chan error
	results chan string

	currentUserID  string
	playlistTracks map[spotify.ID][]spotify.ID
	playlists      map[string]spotify.ID
}

func NewSpotify(clientID string, clientSecret string) *Spotify {
	return &Spotify{
		clientID:     clientID,
		clientSecret: clientSecret,

		errors:  make(chan error),
		results: make(chan string),

		playlists:      make(map[string]spotify.ID),
		playlistTracks: make(map[spotify.ID][]spotify.ID),
	}
}

// Only used in testing to bypass authentication.
func NewSpotifyWithClient(userID string, sc SpotifyClient) *Spotify {
	return &Spotify{
		client:         sc,
		currentUserID:  userID,
		playlists:      make(map[string]spotify.ID),
		playlistTracks: make(map[spotify.ID][]spotify.ID),
	}
}

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)

	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func EncodeBase64WithoutPadding(input []byte) string {
	encoded := base64.URLEncoding.EncodeToString(input)
	parts := strings.Split(encoded, "=")
	encoded = parts[0]

	return encoded
}

func GenerateRandomString(s int) (string, error) {
	b, err := GenerateRandomBytes(s)
	if err != nil {
		return "", err
	}

	return EncodeBase64WithoutPadding(b), nil
}

func (c *Spotify) StartAuthentication() error {
	c.authenticator = spotify.NewAuthenticator(redirectURI,
		spotify.ScopeUserReadPrivate,
		spotify.ScopePlaylistReadPrivate,
		spotify.ScopePlaylistModifyPublic,
		spotify.ScopePlaylistModifyPrivate)

	c.authenticator.SetAuthInfo(c.clientID, c.clientSecret)

	http.HandleFunc("/callback", c.completeAuthentication)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})

	go http.ListenAndServe(":8080", nil)

	state, err := GenerateRandomString(numRandomBytes)
	if err != nil {
		return err
	}

	c.state = state

	randomBytes, err := GenerateRandomBytes(numRandomBytes)
	if err != nil {
		log.Fatal(err)
	}

	c.codeVerifier = EncodeBase64WithoutPadding(randomBytes)

	data := sha256.Sum256([]byte(c.codeVerifier))
	c.codeChallenge = EncodeBase64WithoutPadding(data[:])

	url := c.authenticator.AuthURLWithOpts(state,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", c.codeChallenge),
	)

	fmt.Printf("open the following URL in your browser: %s\n", url)
	browser.OpenURL(url)

	return nil
}

func (c *Spotify) AddTracksToNamedPlaylist(playlistName string, trackIDsToAdd []string) error {
	if len(trackIDsToAdd) > 0 {
		playlistID, err := c.findOrCreatePlaylist(playlistName)
		if err != nil {
			return err
		}

		err = c.updatePlaylist(string(playlistID), trackIDsToAdd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Spotify) updatePlaylist(inputPlaylistID string, inputTrackIDs []string) error {
	playlistID := spotify.ID(inputPlaylistID)

	trackIDs, err := c.cleanTrackIDs(inputTrackIDs)
	if err != nil {
		log.Fatal(err)
	}

	// remove tracks that have been removed from the local playlist
	trackIDsToRemove := c.getTrackIDsToRemove(playlistID, trackIDs)

	err = c.removeTracksFromPlaylist(playlistID, trackIDsToRemove)
	if err != nil {
		log.Fatal(err)
	}

	// add all tracks missing on remote playlist
	trackIDsToAdd := c.getTrackIDsToAdd(playlistID, trackIDs)

	err = c.addTracksToPlaylist(playlistID, trackIDsToAdd)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func (c *Spotify) populatePlaylists() error {
	// only populate local playlist data once
	if len(c.playlistTracks) > 0 && len(c.playlists) > 0 {
		return nil
	}

	limit := maxPlaylists
	offset := 0

	opt := &spotify.Options{
		Limit:  &limit,
		Offset: &offset,
	}

	for {
		playlists, err := c.client.GetPlaylistsForUserOpt(c.currentUserID, opt)
		if err != nil {
			return err
		}

		for _, p := range playlists.Playlists {
			// safety check to only touch playlists with Spotitable prefix
			if !strings.HasPrefix(p.Name, fmt.Sprintf("%s-", playlistPrefix)) {
				continue
			}

			c.playlists[p.Name] = p.ID

			playlistTracksLimit := maxPlaylistTracks
			playlistTracksOffset := 0
			playlistTracksOpt := &spotify.Options{
				Limit:  &playlistTracksLimit,
				Offset: &playlistTracksOffset,
			}

			for {
				playlistTracks, err := c.client.GetPlaylistTracksOpt(p.ID, playlistTracksOpt, "")
				if err != nil {
					return err
				}

				for _, t := range playlistTracks.Tracks {
					c.playlistTracks[p.ID] = append(c.playlistTracks[p.ID], t.Track.ID)
				}

				playlistTracksOffset += playlistTracksLimit

				if playlistTracks.Next == "" || playlistTracksOffset >= playlistTracks.Total {
					break
				}
			}
		}

		offset += limit

		if playlists.Next == "" || offset >= playlists.Total {
			break
		}
	}

	return nil
}

func (c *Spotify) findOrCreatePlaylist(name string) (spotify.ID, error) {
	err := c.populatePlaylists()
	if err != nil {
		return "", err
	}

	// TODO: this script assumes Spotify playlists have unique names, but that is
	// not necessarily true. Consider warninng if there are multiple playlists with
	// the same name.
	playlistID, ok := c.playlists[name]
	if !ok {
		playlist, err := c.client.CreatePlaylistForUser(c.currentUserID, name, "Managed by Spotitable", false)
		if err != nil {
			fmt.Printf("Error: %v", err)

			return "", err
		}

		fmt.Printf("create spotify:playlist:%s (%s)\n", playlist.ID, name)

		return playlist.ID, nil
	}

	return playlistID, nil
}

// Verify that all tracks to be added to the playlist do exist.
func (c *Spotify) cleanTrackIDs(inputTrackIDs []string) ([]spotify.ID, error) {
	trackIDs := []spotify.ID{}

	// convert string IDs to spotify.IDs
	trackIDsToCheck := []spotify.ID{}
	for _, trackID := range inputTrackIDs {
		trackIDsToCheck = append(trackIDsToCheck, spotify.ID(trackID))
	}

	for i := 0; i < len(trackIDsToCheck); i += maxTracks {
		j := i + maxTracks
		if j > len(trackIDsToCheck) {
			j = len(trackIDsToCheck)
		}

		tracks, err := c.client.GetTracks(trackIDsToCheck[i:j]...)
		if err != nil {
			return nil, err
		}

		// do not attempt tot add tracks that don't exist anymore
		for k, t := range tracks {
			if t != nil {
				trackIDs = append(trackIDs, t.ID)
			} else {
				fmt.Printf("spotify:track:%s does not exist", inputTrackIDs[k])
			}
		}
	}

	return trackIDs, nil
}

// Add all tracks missing on remote playlist.
func (c *Spotify) getTrackIDsToAdd(playlistID spotify.ID, inputTrackIDs []spotify.ID) []spotify.ID {
	trackIDsToAdd := []spotify.ID{}

	for _, trackID := range inputTrackIDs {
		found := false

		for _, t := range c.playlistTracks[playlistID] {
			if t == trackID {
				found = true

				break
			}
		}

		if !found {
			fmt.Printf("add spotify:track:%s to spotify:playlist:%s\n", trackID, playlistID)
			trackIDsToAdd = append(trackIDsToAdd, trackID)
		}
	}

	return trackIDsToAdd
}

func (c *Spotify) getTrackIDsToRemove(playlistID spotify.ID, inputTrackIDs []spotify.ID) []spotify.ID {
	var trackIDsToRemove []spotify.ID

	for _, trackID := range c.playlistTracks[playlistID] {
		found := false

		for _, t := range inputTrackIDs {
			if t == trackID {
				found = true

				break
			}
		}

		if !found {
			fmt.Printf("remove spotify:track:%s from spotify:playlist:%s\n", trackID, playlistID)
			trackIDsToRemove = append(trackIDsToRemove, trackID)
		}
	}

	return trackIDsToRemove
}

func (c *Spotify) removeTracksFromPlaylist(playlistID spotify.ID, trackIDs []spotify.ID) error {
	for i := 0; i < len(trackIDs); i += maxPlaylistTracks {
		j := i + maxPlaylistTracks
		if j > len(trackIDs) {
			j = len(trackIDs)
		}

		_, err := c.client.RemoveTracksFromPlaylist(playlistID, trackIDs[i:j]...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Spotify) addTracksToPlaylist(playlistID spotify.ID, trackIDs []spotify.ID) error {
	for i := 0; i < len(trackIDs); i += maxPlaylistTracks {
		j := i + maxPlaylistTracks
		if j > len(trackIDs) {
			j = len(trackIDs)
		}

		_, err := c.client.AddTracksToPlaylist(playlistID, trackIDs[i:j]...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Spotify) completeAuthentication(w http.ResponseWriter, r *http.Request) {
	errors, ok := r.URL.Query()["error"]
	if ok {
		fmt.Fprintf(w, "<html><body><p><strong>Error</strong>: %s</p></body></html>", errors[0])
		c.errors <- fmt.Errorf("error: %s", errors[0])

		return
	}

	token, err := c.authenticator.TokenWithOpts(c.state, r, oauth2.SetAuthURLParam("code_verifier", c.codeVerifier))
	if err != nil {
		http.Error(w, "could not get token", http.StatusForbidden)
		log.Fatal(err)
	}

	client := c.authenticator.NewClient(token)

	user, err := client.CurrentUser()
	if err != nil {
		log.Fatal(err)
	}

	c.client = &client
	c.currentUserID = user.ID

	fmt.Fprintf(w, "<html><body><p>Authenticated as: <strong>%s</strong></p><p>Return to your terminal to continue.</p></body></html>", c.currentUserID)

	c.results <- user.ID
}
