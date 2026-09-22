// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

type BadItem int

const (
	FineItem    BadItem = 0
	BadOverview BadItem = 1
	BadTitle    BadItem = 2
	BadImage    BadItem = 3
	BadAll      BadItem = 4
)

type RemoteImage struct {
	ProviderName string `json:"ProviderName"`
	Url          string `json:"Url"`
	Width        int    `json:"Width"`
	Height       int    `json:"Height"`
	Type         string `json:"Type"`
}
type RemoteImagesResponse struct {
	Images []RemoteImage `json:"Images"`
}

var genericTitlePattern = regexp.MustCompile(`(?i)^\s*(episode|folge|épisode|episodio|epis[oó]dio|aflevering)\s*0*\d+\s*$`)

func hasGenericTitle(name string) bool {
	return genericTitlePattern.MatchString(strings.TrimSpace(name))
}

func loadConfig() Config {
	configDir, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		configDir = "~/.config"
	}
	file, err := os.Open(configDir + "/jellyfin-autorefresh-new-releases/config.json")
	if err != nil {
		log.Fatalln("Could not load config from " + configDir + "/jellyfin-autorefresh-new-releases/config.json. Quitting!")
	}
	defer file.Close()

	config := Config{DesiredImageHeight: 360, DaysToScan: 2} //Default value
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatalln("Error reading config:", err)
	}

	u, err := url.ParseRequestURI(config.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		log.Fatalln("Invalid URL was provided!")
	}
	if config.APIKey == "" {
		log.Fatalln("Empty API key was provided!")
	}
	if config.DaysToScan > 14 {
		config.DaysToScan = 14 // Sensible upper limit
	}

	return config
}

func fetchItems(client *http.Client, cfg *Config, params *url.Values) []Item {
	req, err := http.NewRequest("GET", cfg.URL+"/Items", nil)
	if err != nil {
		log.Fatalln(err)
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+cfg.APIKey+`"`)
	req.URL.RawQuery = params.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	if !isSuccess(resp) {
		log.Fatalln("Request failed. Please check the API key. \nError:", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	var parsed ItemsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Fatalln(err)
	}

	items := parsed.Items
	slices.SortFunc(parsed.Items, func(a, b Item) int {
		return a.PremiereDate.Compare(*b.PremiereDate)
	})
	return items
}

func isItemFine(client *http.Client, config *Config, item *Item) BadItem {
	itemStatus := FineItem
	if hasGenericTitle(item.Name) {
		log.Println(Red + "     Title looks like a generic placeholder." + Reset)
		itemStatus = BadTitle
	}
	if strings.TrimSpace(item.Overview) == "" {
		log.Println(Red + "     Overview is missing." + Reset)
		itemStatus = BadOverview
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/Items/%s/Images", config.URL, item.ID), nil)
	if err != nil {
		log.Println(Red+"  Request creation failed:", err, Reset)
		if itemStatus == FineItem {
			itemStatus = BadImage
		} else {
			itemStatus = BadAll
		}
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+config.APIKey+`"`)
	resp, err := client.Do(req)
	if err != nil {
		log.Println(Red+"  Error getting info about item", item, Reset)
		itemStatus = BadTitle
	}
	defer resp.Body.Close()

	if isSuccess(resp) {
		var images []ImageList
		json.NewDecoder(resp.Body).Decode(&images)
		if len(images) <= 0 {
			log.Println(Red + "     Primary image is missing." + Reset)
			itemStatus = BadImage
		}
		for _, image := range images {
			if image.Type == "Primary" {
				if image.Height < config.DesiredImageHeight {
					log.Printf(Red+"     Primary image is of low quality (%dx%d)."+Reset, image.Width, image.Height)
					if itemStatus == FineItem {
						itemStatus = BadImage
					} else {
						itemStatus = BadAll
					}
				} else {
					itemStatus = FineItem
					break
				}
			}
		}
	} else {
		log.Println(Red + "     Primary image is missing." + Reset)
		itemStatus = BadImage
	}

	return itemStatus
}

func getRemoteImages(
	client *http.Client,
	config *Config,
	item *Item,
) ([]RemoteImage, error) {

	params := url.Values{}
	params.Set("Type", "Primary")
	params.Set("Limit", "100")

	endpoint := fmt.Sprintf(
		"%s/Items/%s/RemoteImages?%s",
		config.URL,
		item.ID,
		params.Encode(),
	)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set(
		"Authorization",
		`MediaBrowser Token="`+config.APIKey+`"`,
	)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !isSuccess(resp) {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var result RemoteImagesResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Images, nil
}

func getBestImage(images []RemoteImage) *RemoteImage {
	if len(images) == 0 {
		return nil
	}

	best := &images[0]
	bestPixels := best.Width * best.Height

	var tmdbImages []*RemoteImage
	for _, image := range images {
		pixels := image.Width * image.Height
		if pixels > bestPixels {
			best = &image
			bestPixels = pixels
		}
		if image.ProviderName == "TheMovieDb" {
			tmdbImages = append(tmdbImages, &image)
		}
	}

	if bestPixels == 0 {
		if len(tmdbImages) > 0 {
			return tmdbImages[rand.Intn(len(tmdbImages))]
		}
		return &images[rand.Intn(len(images))]
	}

	return best
}

func setRemoteImage(
	client *http.Client,
	config *Config,
	item *Item,
	image *RemoteImage,
) error {

	params := url.Values{}

	params.Set("Type", image.Type)
	params.Set("ImageUrl", image.Url)

	endpoint := fmt.Sprintf(
		"%s/Items/%s/RemoteImages/Download?%s",
		config.URL,
		item.ID,
		params.Encode(),
	)

	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set(
		"Authorization",
		`MediaBrowser Token="`+config.APIKey+`"`,
	)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !isSuccess(resp) {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return nil
}

func refreshItem(client *http.Client, config *Config, item *Item, itemStatus BadItem) error {
	needToCheck := false
	respStatus := "ok"

	if itemStatus == BadTitle || itemStatus == BadOverview || itemStatus == BadAll {
		updateParams := url.Values{}
		updateParams.Add("metadataRefreshMode", "FullRefresh")
		updateParams.Add("replaceAllMetadata", "true")

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/Items/%s/Refresh", config.URL, item.ID), nil)
		if err != nil {
			log.Println(Red+"  Request creation failed:", err, Reset)
			return err
		}
		req.Header.Set("Authorization", `MediaBrowser Token="`+config.APIKey+`"`)
		req.URL.RawQuery = updateParams.Encode()

		resp, err := client.Do(req)
		if err != nil {
			log.Println(Red+"  Refresh failed:", err, Reset)
			return err
		}
		defer resp.Body.Close()
		needToCheck = isSuccess(resp)
		respStatus = resp.Status
	}

	if itemStatus == BadImage || itemStatus == BadAll {
		images, err := getRemoteImages(client, config, item)
		if err != nil {
			return err
		}

		best := getBestImage(images)

		if best == nil {
			log.Println(Red + "    No remote images found." + Reset)
			return errors.New("No remote images found")
		}

		if best.Width > 0 && best.Height > 0 {
			fmt.Printf(
				"     Selected image: %dx%d (%s)\n",
				best.Width,
				best.Height,
				best.ProviderName,
			)
		} else {
			fmt.Printf(
				"     Selected image: Unknown dimensions (%s)\n",
				best.ProviderName,
			)
		}
		if err := setRemoteImage(client, config, item, best); err != nil {
			return err
		}
		needToCheck = true
	}

	if needToCheck {
		// Wait five seconds so that the metadata is actually updated
		time.Sleep(5 * time.Second)
		// Check if the update was successful
		queryParams := url.Values{}
		queryParams.Add("ids", item.ID)
		queryParams.Add("fields", "Overview")
		updatedItem := fetchItems(client, config, &queryParams)[0]
		if isItemFine(client, config, &updatedItem) == FineItem {
			fmt.Println("     Refresh successful!")
			fmt.Println(Green + "     The episode now satisfies all the desired criteria.\n" + Reset)
			return nil
		} else {
			fmt.Println(Red + "     The desired criteria are still not met." + Reset)
			return errors.New("No new data.")
		}
	} else {
		fmt.Println(Red+"     Refresh failed:", respStatus, Reset)
		return errors.New("HTTP Error " + respStatus)
	}
}

func isSuccess(resp *http.Response) bool {
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
