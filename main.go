// SPDX-FileCopyrightText: 2025 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Config struct {
	Key     string `json:"key"`
	BaseURI string `json:"baseURI"`
}

type Item struct {
	ID         string `json:"Id"`
	Name       string `json:"Name"`
	SeriesName string `json:"SeriesName"`
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
	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatalln("Error reading config:", err)
	}

	return config
}

func main() {
	config := loadConfig()

	client := &http.Client{}
	cutoffDate := time.Now().AddDate(0, 0, -3).Format(time.RFC3339)

	// Get all items released in the last two days
	params := url.Values{}
	params.Add("includeItemTypes", "Episode")
	params.Add("recursive", "true")
	params.Add("minPremiereDate", cutoffDate)
	dataAll := fetchItems(client, config, params)
	// Get only those with proper info
	params.Add("hasOverview", "true")
	params.Add("imageTypes", "Primary")
	dataWithImages := fetchItems(client, config, params)
	// Figure out the episodes with missing info
	idsWithImages := make(map[string]bool)
	for _, item := range dataWithImages {
		idsWithImages[item.ID] = true
	}

	fmt.Printf("Processing all episodes released in the last two days.\n\n")
	var successCount, failCount int
	firstItem := true
	for _, item := range dataAll {
		if !firstItem {
			// Wait a second before the next request to not reach any rate limits
			time.Sleep(time.Second)
			firstItem = false
		}
		fmt.Printf("  %s %s : %s\n", item.ID, item.Name, item.SeriesName)

		if idsWithImages[item.ID] {
			fmt.Printf("  All desired properties are present. Skipping...\n\n")
			continue
		} else {
			fmt.Println("  Some desired properties are missing. Requesting a refresh...")
		}

		if refreshItem(client, config, item.ID) {
			successCount++
		} else {
			fmt.Println("  Retrying in 2 seconds...")
			time.Sleep(2 * time.Second)
			if refreshItem(client, config, item.ID) {
				successCount++
			} else {
				failCount++
				fmt.Printf("  Not trying again!\n\n")
			}
		}
	}
	// Print a summary
	fmt.Println("Summary:")
	fmt.Println("  Successful refreshes:", successCount)
	fmt.Println("  Failed refreshes:", failCount)
}

func fetchItems(client *http.Client, cfg Config, params url.Values) []Item {
	req, err := http.NewRequest("GET", cfg.BaseURI+"/Items", nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+cfg.Key+`"`)
	req.URL.RawQuery = params.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var parsed ItemsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Fatal(err)
	}
	return parsed.Items
}

func refreshItem(client *http.Client, cfg Config, id string) bool {
	params := url.Values{}
	params.Add("metadataRefreshMode", "FullRefresh")
	params.Add("imageRefreshMode", "FullRefresh")
	params.Add("replaceAllMetadata", "true")
	params.Add("replaceAllImages", "true")

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/Items/%s/Refresh", cfg.BaseURI, id), nil)
	if err != nil {
		log.Println("Request creation failed:", err)
		return false
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+cfg.Key+`"`)
	req.URL.RawQuery = params.Encode()

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Refresh failed:", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("  Refresh successful!\n\n")
		return true
	}

	fmt.Println("  Refresh failed:", resp.Status)
	return false
}
