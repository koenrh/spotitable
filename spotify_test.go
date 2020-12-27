package main

import (
	"testing"

	"github.com/zmb3/spotify"
)

func TestEncodeBase64WithoutPadding(t *testing.T) {
	input := []byte{112, 105, 122, 122, 97}

	expected := "cGl6emE"
	got := EncodeBase64WithoutPadding(input)

	if got != expected {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func NewTestSpotify() *TestSpotify {
	return &TestSpotify{
		remotePlaylistsTracks: make(map[string][]string),
	}
}

type TestSpotify struct {
	remotePlaylistsTracks map[string][]string
}

func (s *TestSpotify) SetTracks(playlistTracks map[string][]string) {
	s.remotePlaylistsTracks = playlistTracks
}

func (s *TestSpotify) GetTracks(tracks ...spotify.ID) ([]*spotify.FullTrack, error) {
	newTracks := []*spotify.FullTrack{}

	for _, t := range tracks {
		newTracks = append(newTracks, &spotify.FullTrack{
			SimpleTrack: spotify.SimpleTrack{ID: t},
		})
	}

	return newTracks, nil
}

// Adds tracks to in-memory store.
func (s *TestSpotify) AddTracksToPlaylist(pID spotify.ID, tracks ...spotify.ID) (string, error) {
	for _, t := range tracks {
		s.remotePlaylistsTracks[string(pID)] = append(s.remotePlaylistsTracks[string(pID)], string(t))
	}

	return "", nil
}

func (s *TestSpotify) RemoveTracksFromPlaylist(playlistID spotify.ID, tracks ...spotify.ID) (string, error) {
	newTrackIDs := []string{}

	for _, t := range s.remotePlaylistsTracks[string(playlistID)] {
		found := false

		for _, x := range tracks {
			if string(x) == t {
				found = true

				break
			}
		}

		if !found {
			newTrackIDs = append(newTrackIDs, t)
		}
	}

	s.remotePlaylistsTracks[string(playlistID)] = newTrackIDs

	return "", nil
}

func (s *TestSpotify) CreatePlaylistForUser(string, string, string, bool) (*spotify.FullPlaylist, error) {
	return &spotify.FullPlaylist{
		SimplePlaylist: spotify.SimplePlaylist{
			ID:   "spotify:playlist:1",
			Name: "st-foo",
		},
		Tracks: spotify.PlaylistTrackPage{},
	}, nil
}

func (s *TestSpotify) GetPlaylistTracksOpt(playlistID spotify.ID, opts *spotify.Options, sth string) (*spotify.PlaylistTrackPage, error) {
	tracks := []spotify.PlaylistTrack{}

	for _, t := range s.remotePlaylistsTracks[string(playlistID)] {
		tracks = append(tracks,
			spotify.PlaylistTrack{
				Track: spotify.FullTrack{
					SimpleTrack: spotify.SimpleTrack{
						ID: spotify.ID(t),
					},
				},
			},
		)
	}

	return &spotify.PlaylistTrackPage{Tracks: tracks}, nil
}

func (s *TestSpotify) GetPlaylistsForUserOpt(pizza string, opt *spotify.Options) (*spotify.SimplePlaylistPage, error) {
	return &spotify.SimplePlaylistPage{
		Playlists: []spotify.SimplePlaylist{
			{ID: "spotify:playlist:foo1", Name: "st-foo", Tracks: spotify.PlaylistTracks{}},
		},
	}, nil
}

func TestAddTracksToPlaylistAddTracks(t *testing.T) {
	testSpotify := NewTestSpotify()
	spotifyClient := NewSpotifyWithClient("koenrh", testSpotify)

	_ = spotifyClient.AddTracksToNamedPlaylist("st-foo", []string{"spotify:track:baz1", "spotify:track:baz2"})
	playlist, _ := testSpotify.GetPlaylistTracksOpt("spotify:playlist:foo1", &spotify.Options{}, "")

	expected := []string{"spotify:track:baz1", "spotify:track:baz2"}

	if len(expected) != len(playlist.Tracks) {
		t.Fatalf("different number of results, expected: %d, got %d", len(expected), len(playlist.Tracks))
	}

	for i, p := range playlist.Tracks {
		if expected[i] != string(p.Track.ID) {
			t.Fatalf("IDs don't match, expected: %s, got: %s", expected[i], string(p.Track.ID))
		}
	}
}

func TestAddTracksToPlaylistRemoveTracks(t *testing.T) {
	testSpotify := NewTestSpotify()
	spotifyClient := NewSpotifyWithClient("koenrh", testSpotify)
	testSpotify.SetTracks(map[string][]string{
		"spotify:playlist:foo1": {"spotify:track:rofl", "spotify:track:foo2"},
	})

	_ = spotifyClient.AddTracksToNamedPlaylist("st-foo", []string{"spotify:track:foo1", "spotify:track:foo2"})
	playlist, _ := testSpotify.GetPlaylistTracksOpt("spotify:playlist:foo1", &spotify.Options{}, "")

	expected := []string{"spotify:track:foo2", "spotify:track:foo1"}

	if len(expected) != len(playlist.Tracks) {
		t.Fatalf("different number of results, expected: %d, got %d", len(expected), len(playlist.Tracks))
	}

	for i, p := range playlist.Tracks {
		if expected[i] != string(p.Track.ID) {
			t.Fatalf("IDs don't match, expected: %s, got: %s", expected[i], string(p.Track.ID))
		}
	}
}
