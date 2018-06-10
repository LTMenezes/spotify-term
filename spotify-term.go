package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
)

// GetAPITokenResponse represents /api/token spotify api response.
type GetAPITokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// CurrentlyPlayingResponse represents v1/me/player spotify api response.
type CurrentlyPlayingResponse struct {
	Timestamp  int64       `json:"timestamp"`
	Context    interface{} `json:"context"`
	ProgressMs int         `json:"progress_ms"`
	Item       struct {
		Album struct {
			AlbumType string `json:"album_type"`
			Artists   []struct {
				ExternalUrls struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Href string `json:"href"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
				URI  string `json:"uri"`
			} `json:"artists"`
			AvailableMarkets []interface{} `json:"available_markets"`
			ExternalUrls     struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
			Href   string `json:"href"`
			ID     string `json:"id"`
			Images []struct {
				Height int    `json:"height"`
				URL    string `json:"url"`
				Width  int    `json:"width"`
			} `json:"images"`
			Name                 string `json:"name"`
			ReleaseDate          string `json:"release_date"`
			ReleaseDatePrecision string `json:"release_date_precision"`
			Type                 string `json:"type"`
			URI                  string `json:"uri"`
		} `json:"album"`
		Artists []struct {
			ExternalUrls struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
			Href string `json:"href"`
			ID   string `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
			URI  string `json:"uri"`
		} `json:"artists"`
		AvailableMarkets []interface{} `json:"available_markets"`
		DiscNumber       int           `json:"disc_number"`
		DurationMs       int           `json:"duration_ms"`
		Explicit         bool          `json:"explicit"`
		ExternalIds      struct {
			Isrc string `json:"isrc"`
		} `json:"external_ids"`
		ExternalUrls struct {
			Spotify string `json:"spotify"`
		} `json:"external_urls"`
		Href        string      `json:"href"`
		ID          string      `json:"id"`
		IsLocal     bool        `json:"is_local"`
		Name        string      `json:"name"`
		Popularity  int         `json:"popularity"`
		PreviewURL  interface{} `json:"preview_url"`
		TrackNumber int         `json:"track_number"`
		Type        string      `json:"type"`
		URI         string      `json:"uri"`
	} `json:"item"`
	IsPlaying bool `json:"is_playing"`
}

// GetDevicesResponse represents v1/me/player/devices spotify api response.
type GetDevicesResponse struct {
	Devices []struct {
		ID               string `json:"id"`
		IsActive         bool   `json:"is_active"`
		IsPrivateSession bool   `json:"is_private_session"`
		IsRestricted     bool   `json:"is_restricted"`
		Name             string `json:"name"`
		Type             string `json:"type"`
		VolumePercent    int    `json:"volume_percent"`
	} `json:"devices"`
}

// SpotifyTermConfig is a spotify term configuration file representation.
type SpotifyTermConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectPort string `json:"redirect_port"`
}

func getConfig() (config SpotifyTermConfig, err error) {
	path, _ := homedir.Dir()
	configFilePath := strings.Join([]string{path, "\\.spotify-term.config"}, "")
	configFile, _ := os.OpenFile(configFilePath, os.O_RDONLY|os.O_CREATE, 0644)

	reader := bufio.NewReader(configFile)
	fileContent, _ := reader.Peek(1000)
	fileContentAsString := string(fileContent)
	configFile.Close()

	config = SpotifyTermConfig{}

	if fileContentAsString == "" {
		reader := bufio.NewReader(os.Stdin)

		fmt.Println("Welcome to spotify term, we need to perform some first time setup.")
		fmt.Println("In order to avoid using a third-party server and host all the code on your computer you need to create a spotify application in the link:")
		fmt.Println("https://developer.spotify.com")
		fmt.Println("After creating it enter the following settings with your spotify aplication information:")

		fmt.Print("Enter your client ID:")
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error getting user configuration.")
			fmt.Println(err)
			return config, err
		}
		text = strings.TrimSuffix(text, "\n")
		config.ClientID = strings.TrimSuffix(text, "\r")

		fmt.Print("Enter your client secret:")
		text, err = reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error getting user configuration.")
			fmt.Println(err)
			return config, err
		}
		text = strings.TrimSuffix(text, "\n")
		config.ClientSecret = strings.TrimSuffix(text, "\r")

		fmt.Print("Enter the desired port for authentification redirect [5958]:")
		text, err = reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error getting user configuration.")
			fmt.Println(err)
			return config, err
		}
		text = strings.TrimSuffix(text, "\n")
		config.RedirectPort = strings.TrimSuffix(text, "\r")

		configFile, err = os.OpenFile(configFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("Error saving user configuration.")
			fmt.Println(err)
			return config, err
		}

		json, err := json.Marshal(config)
		if err != nil {
			fmt.Println("Error saving user configuration.")
			fmt.Println(err)
			return config, err
		}

		_, err = configFile.Write([]byte(json))
		if err != nil {
			fmt.Println("Error saving user configuration.")
			fmt.Println(err)
			return config, err
		}

		configFile.Close()

		redirectURI := fmt.Sprintf("http://localhost:%s/callback", config.RedirectPort)
		fmt.Println("The setup is done. Don't forget to add the following URI to spotify application redirect URIs: ", redirectURI)
	} else {
		err := json.Unmarshal(fileContent, &config)
		if err != nil {
			fmt.Println("Error getting user configuration.")
			fmt.Println(err)
			return config, err
		}
	}

	return
}

func getAuthorizationCode() (authorizationCode string, err error) {
	redirectURI := "http://localhost:%s/callback"
	authorizationURI := "https://accounts.spotify.com/authorize?scope=user-modify-playback-state%%20user-read-currently-playing%%20user-read-playback-state&client_id=%s&response_type=code&redirect_uri=%s"

	config, err := getConfig()
	if err != nil {
		fmt.Println("Error loading config.")
		return
	}

	srv := &http.Server{Addr: ":" + config.RedirectPort}

	authorizationCode = ""

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query["code"] != nil && len(query["code"]) != 0 {
			authorizationCode = query["code"][0]

			io.WriteString(w, "Sucesssfuly authorized, you can go back to your terminal now.\n")
		} else {
			io.WriteString(w, "Couldn't get code from request URI.\n")
		}
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil {
		}
	}()

	redirectURI = fmt.Sprintf(redirectURI, config.RedirectPort)
	clientAuthString, _ := url.Parse(fmt.Sprintf(authorizationURI, config.ClientID, redirectURI))

	fmt.Println("Please, enter this link in your browser to authorize the app: ", clientAuthString.String())

	for authorizationCode == "" {
		// Spin lock to await client redirect.
	}

	return
}

func getAPIToken() (accessToken string, err error) {
	redirectURI := "http://localhost:%s/callback"
	apiTokenURI := "https://accounts.spotify.com/api/token"
	authorizationCode := ""
	isRefreshRequest := false

	path, err := homedir.Dir()
	if err != nil {
		fmt.Println("Error getting user home directory.")
		fmt.Println(err)
		return authorizationCode, err
	}

	storageFilePath := strings.Join([]string{path, "\\.spotify-term"}, "")
	storageFile, err := os.OpenFile(storageFilePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("Error opening spotify term storage file.")
		fmt.Println(err)
		return authorizationCode, err
	}

	reader := bufio.NewReader(storageFile)
	fileContent, _ := reader.Peek(1000)
	fileContentAsString := string(fileContent)

	if fileContentAsString == "" {
		authorizationCode, err = getAuthorizationCode()
		if err != nil {
			fmt.Println("Error getting user authorization code.")
			fmt.Println(err)
			return authorizationCode, err
		}

	} else {
		fileContentStruct := new(GetAPITokenResponse)
		err = json.Unmarshal(fileContent, &fileContentStruct)
		if err != nil {
			fmt.Println("Error deserializing spotify term storage file.")
			fmt.Println(err)
			return authorizationCode, err
		}

		authorizationCode = fileContentStruct.RefreshToken
		isRefreshRequest = true
	}
	storageFile.Close()

	config, err := getConfig()
	if err != nil {
		fmt.Println("Error loading config.")
		fmt.Println(err)
		return
	}
	redirectURI = fmt.Sprintf(redirectURI, config.RedirectPort)

	data := url.Values{}
	if isRefreshRequest {
		data.Add("grant_type", "refresh_token")
		data.Add("refresh_token", authorizationCode)
	} else {
		data.Add("grant_type", "authorization_code")
		data.Add("code", authorizationCode)
	}

	data.Add("redirect_uri", redirectURI)

	req, _ := http.NewRequest("POST", apiTokenURI, strings.NewReader(data.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(config.ClientID+":"+config.ClientSecret)))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error on spotify access token request.")
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	responseJSON := new(GetAPITokenResponse)

	json.Unmarshal(body, &responseJSON)

	if isRefreshRequest {
		responseJSON.RefreshToken = authorizationCode
	}

	if responseJSON.RefreshToken != "" && responseJSON.AccessToken != "" {
		storageFile, _ = os.OpenFile(storageFilePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		json, _ := json.Marshal(responseJSON)
		storageFile.Write([]byte(json))
		storageFile.Close()
	}

	accessToken = responseJSON.AccessToken
	return
}

func resumeTrack() {
	resumeTrackURI := "https://api.spotify.com/v1/me/player/play"

	req, _ := http.NewRequest("PUT", resumeTrackURI, strings.NewReader(""))
	accessToken, err := getAPIToken()
	if err != nil {
		fmt.Println("Error getting api access token.")
		fmt.Println(err)
		return
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error on resume track request.")
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	ioutil.ReadAll(resp.Body)

	displayCurrentPlayingTrack()
}

func pauseTrack() {
	pauseTrackURI := "https://api.spotify.com/v1/me/player/pause"

	req, _ := http.NewRequest("PUT", pauseTrackURI, strings.NewReader(""))
	accessToken, err := getAPIToken()
	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error getting api access token.")
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	ioutil.ReadAll(resp.Body)

	fmt.Println("Track paused.")
}

func skipToNextTrack() {
	skipToNextTrackURI := "https://api.spotify.com/v1/me/player/next"

	req, _ := http.NewRequest("POST", skipToNextTrackURI, strings.NewReader(""))
	accessToken, err := getAPIToken()
	if err != nil {
		fmt.Println("Error getting api access token.")
		fmt.Println(err)
		return
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error on skip to next track api request.")
		fmt.Println(err)
		return
	}

	defer resp.Body.Close()

	ioutil.ReadAll(resp.Body)

	time.Sleep(1 * time.Second)
	displayCurrentPlayingTrack()
}

func skipToPreviousTrack() {
	skipToPreviousTrackURI := "https://api.spotify.com/v1/me/player/previous"

	req, _ := http.NewRequest("POST", skipToPreviousTrackURI, strings.NewReader(""))
	accessToken, err := getAPIToken()
	if err != nil {
		fmt.Println("Error getting api access token.")
		fmt.Println(err)
		return
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error on skip to previous track api request.")
		fmt.Println(err)
		return
	}

	defer resp.Body.Close()

	ioutil.ReadAll(resp.Body)

	time.Sleep(1 * time.Second)
	displayCurrentPlayingTrack()
}

func displayCurrentPlayingTrack() {
	getCurrentPlayingURI := "https://api.spotify.com/v1/me/player/currently-playing"

	req, _ := http.NewRequest("GET", getCurrentPlayingURI, strings.NewReader(""))
	accessToken, err := getAPIToken()
	if err != nil {
		fmt.Println("Error getting api access token.")
		fmt.Println(err)
		return
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error on currently playing api request.")
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	response := new(CurrentlyPlayingResponse)
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error on deserializing currently playing api response.")
		fmt.Println(err)
		return
	}

	fmt.Println("Now playing: ", response.Item.Name, " by ", response.Item.Artists[0].Name)
}

func getDevices() {
	getDevicesURI := "https://api.spotify.com/v1/me/player/devices"

	req, _ := http.NewRequest("GET", getDevicesURI, strings.NewReader(""))
	accessToken, err := getAPIToken()
	if err != nil {
		fmt.Println("Error getting api access token.")
		fmt.Println(err)
		return
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error on get devices request.")
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	response := new(GetDevicesResponse)
	err = json.Unmarshal(body, &response)

	if err != nil {
		fmt.Println("Error on deserializing get devices api response.")
		fmt.Println(err)
		return
	}

	if len(response.Devices) == 0 {
		fmt.Println("There are no available devices at the moment.")
	}

	for i := 0; i < len(response.Devices); i++ {
		fmt.Println(response.Devices[i].Name, " ", response.Devices[i].Type, " ", response.Devices[i].VolumePercent, " ", response.Devices[i].ID)
	}
}

func getMe() {
	getMeURI := "https://api.spotify.com/v1/me"

	req, _ := http.NewRequest("GET", getMeURI, strings.NewReader(""))
	accessToken, err := getAPIToken()
	if err != nil {
		fmt.Println("Error getting api access token.")
		fmt.Println(err)
		return
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error on get devices request.")
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	fmt.Println(string(body))
}

func main() {
	callArguments := os.Args[1:]

	if len(callArguments) == 0 {
		fmt.Println("Enter a valid command. Enter --help for help.")
		return
	}

	switch callArguments[0] {
	case "setup":
		getConfig()
		return

	case "login":
		_, err := getAPIToken()
		if err == nil {
			fmt.Println("Sucessfully logged in!")
		}
		return

	case "devices":
		getDevices()
		return

	case "next":
		skipToNextTrack()
		return

	case "previous":
		skipToPreviousTrack()
		return

	case "pause":
		pauseTrack()
		return

	case "resume":
		resumeTrack()
		return

	case "--help", "-h":
		fmt.Println("available functions:")
		fmt.Println("setup - Perform first time setup.")
		fmt.Println("login - Authorize application on spotify.")
		fmt.Println("resume - Resume current track.")
		fmt.Println("pause - Pause current track.")
		fmt.Println("next - Skip to next track.")
		fmt.Println("previous - Skip to next track.")
		fmt.Println("devices - Show current connected devices.")
		fmt.Println("")
		fmt.Println("spotify-term uses two configurations files: \"~\\.spotify-term\" and \"~\\.spotify-term.config\". You are free to delete them to force a reconfiguration or make any changes you see fit.")
		return

	default:
		fmt.Println("Invalid command. Enter --help for help.")
		return
	}
}
