# spotitable

Dynamically create and update playlists in Spotify based on your music database
in Airtable.

## Prerequisites

### Create Spotify app

1. [Create a new Spotify app](https://developer.spotify.com/dashboard/applications)
1. Add a 'name' and 'description'
1. Click 'edit settings'
1. Add `http://127.0.0.1:8080/callback` as a 'Redirect URI' and and click 'save'
1. Note the 'Client ID' and 'Client Secret' (click 'show client secret') on the
  app details page

### Create Airtable music base

1. Create a new Airtable base with a 'Tracks' table ([example](https://airtable.com/shr0fWC6pIoGSIFjw/tbllF33I2YDE6GnyO))
1. Make sure that the table at least has the following columns:
    * Year (Number)
    * Like (Checkbox)
    * Love (Checkbox)
    * BPM (Number)
    * Spotify ID (Single line text)
1. [Generate an Airtable API key](https://airtable.com/account)

### Configure environment

:warning: Before you continue, you should be aware that most shells record user
input (and thus secrets) into a history file. In Bash you could prevent this by
prepending your command with a _single space_ (requires `$HISTCONTROL` to be set
to `ignorespace` or `ignoreboth`).

```bash
export AIRTABLE_API_KEY="your_airtable_api_key"
export SPOTIFY_CLIENT_ID="your_spotify_client_id"
export SPOTIFY_CLIENT_SECRET="your_spotify_client_secret"
```

## Installation

```bash
go get github.com/koenrh/spotitable
```

## Usage

Create and update Spotify playlists based on the specified Airtable base and table.

```bash
spotitable -base appdhLGd8xDk23xlP -table "Tracks"
```

In order to interact with the Spotify API, you need to request a Spotify access
token that is authorized to perform actions on your account. In order to retrieve
that token this tool opens the Spotify login page in your default browser.
After your confirmation it will receive the access token.

---

<img width="1440" alt="spotify authentication" src="https://user-images.githubusercontent.com/1307291/103358666-76a59b00-4ab6-11eb-9a18-156776ef8005.png">

<img width="1440" alt="airtable" src="https://user-images.githubusercontent.com/1307291/103284757-67521f00-49dc-11eb-8d21-dcdbb0012470.png">

<img width="1440" alt="spotify playlist" src="https://user-images.githubusercontent.com/1307291/103284976-168ef600-49dd-11eb-9b1f-85879f07dcc2.png">
