// SPDX-FileCopyrightText: 2025 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Config struct {
	APIKey             string `json:"apiKey"`
	URL                string `json:"jellyfinURL"`
	DesiredImageHeight uint16 `json:"desiredImageHeight"`
}

type Item struct {
	ID         string `json:"Id"`
	Name       string `json:"Name"`
	SeriesName string `json:"SeriesName"`
}

type ImageList struct {
	Type   string `json:"ImageType"`
	Height uint16 `json:"Height"`
}

type ItemsResponse struct {
	Items []Item `json:"Items"`
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

	config := Config{DesiredImageHeight: 360} //Default value
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

	return config
}

func main() {
	log.SetFlags(0)
	config := loadConfig()

	client := &http.Client{}
	cutoffDate := time.Now().AddDate(0, 0, -3).Format(time.RFC3339)

	// Get all items released in the last two days
	queryParams := url.Values{}
	queryParams.Add("includeItemTypes", "Episode")
	queryParams.Add("recursive", "true")
	queryParams.Add("minPremiereDate", cutoffDate)
	dataAll := fetchItems(client, config, queryParams)
	// Figure out the episodes with missing info
	idsWithImages := make(map[string]bool)
	for _, item := range dataAll {
		idsWithImages[item.ID] = isItemFine(client, config, item.ID)
	}

	fmt.Println("Jellyfin Autorefresh New Releases (SinTan1729)\n----------")
	fmt.Println("Starting at", time.Now().Format(time.RFC1123))
	fmt.Println("Connecting to", config.URL)
	fmt.Printf("Processing all episodes released in the last two days.\n\n")
	var successCount, failCount, skipCount int
	for i, item := range dataAll {
		fmt.Printf("  %d. ID:%s\n  %s : %s\n", i+1, item.ID, item.Name, item.SeriesName)

		if idsWithImages[item.ID] {
			fmt.Printf("  All desired criteria are met. Skipping.\n\n")
			skipCount++
			continue
		} else {
			fmt.Println("  Some desired criteria are not met. Requesting a refresh...")
		}

		// Wait a second before the next request to not reach any rate limits
		time.Sleep(time.Second)
		if refreshItem(client, config, item.ID) == nil {
			successCount++
		} else {
			fmt.Println("  Retrying in 2 seconds...")
			time.Sleep(2 * time.Second)
			if refreshItem(client, config, item.ID) == nil {
				successCount++
			} else {
				failCount++
				fmt.Printf("  Not trying again!\n\n")
			}
		}
	}
	// Print a summary
	fmt.Println("Summary:")
	fmt.Println("  Skipped:", skipCount)
	fmt.Println("  Successful refreshes:", successCount)
	fmt.Println("  Failed refreshes:", failCount)
	fmt.Printf("----------\n\n")
}

func fetchItems(client *http.Client, cfg Config, params url.Values) []Item {
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
	return parsed.Items
}

func isItemFine(client *http.Client, config Config, id string) bool {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/Items/%s/Images", config.URL, id), nil)
	if err != nil {
		log.Println("  Request creation failed:", err)
		return false
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+config.APIKey+`"`)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("  Error getting info about item", id)
		return false
	}
	defer resp.Body.Close()

	fineFlag := false
	if isSuccess(resp) {
		var images []ImageList
		json.NewDecoder(resp.Body).Decode(&images)
		for _, image := range images {
			fineFlag = image.Type == "Primary" && image.Height >= config.DesiredImageHeight
		}
	}

	return fineFlag
}

func refreshItem(client *http.Client, config Config, id string) error {
	updateParams := url.Values{}
	updateParams.Add("metadataRefreshMode", "FullRefresh")
	updateParams.Add("imageRefreshMode", "FullRefresh")
	updateParams.Add("replaceAllMetadata", "true")
	updateParams.Add("replaceAllImages", "true")

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/Items/%s/Refresh", config.URL, id), nil)
	if err != nil {
		log.Println("  Request creation failed:", err)
		return err
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+config.APIKey+`"`)
	req.URL.RawQuery = updateParams.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Println("  Refresh failed:", err)
		return err
	}
	defer resp.Body.Close()

	if isSuccess(resp) {
		// Wait five seconds so that the metadata is actually updated
		time.Sleep(5 * time.Second)
		// Check if the update was successful
		fmt.Println("  Refresh successful!")
		if isItemFine(client, config, id) {
			fmt.Printf("  The episode now satisfies all the desired criteria.\n\n")
		} else {
			fmt.Printf("  The desired criteria are still not met. Better luck next time!\n\n")
		}
		return nil
	}

	fmt.Println("  Refresh failed:", resp.Status)
	return errors.New(resp.Status)
}

func isSuccess(resp *http.Response) bool {
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
