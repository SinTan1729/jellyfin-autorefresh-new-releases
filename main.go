// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	SeasonNo   uint16 `json:"ParentIndexNumber"`
	EpisodeNo  uint16 `json:"IndexNumber"`
	Overview   string `json:"Overview"`
}

type ImageList struct {
	Type   string `json:"ImageType"`
	Height uint16 `json:"Height"`
}

type ItemsResponse struct {
	Items []Item `json:"Items"`
}

func main() {
	log.SetFlags(0)
	config := loadConfig()

	client := &http.Client{}
	// Get all items released in the last two days
	queryParams := url.Values{}
	queryParams.Add("includeItemTypes", "Episode")
	queryParams.Add("recursive", "true")
	queryParams.Add("fields", "Overview")
	cutoffDate := time.Now().AddDate(0, 0, -3).Format(time.RFC3339)
	queryParams.Add("minPremiereDate", cutoffDate)
	dataAll := fetchItems(client, &config, &queryParams)

	fmt.Println("Jellyfin Autorefresh New Releases (SinTan1729)\n----------")
	fmt.Println("Starting at", time.Now().Format(time.RFC1123))
	fmt.Println("Connecting to", config.URL)
	fmt.Printf("Processing all episodes released in the last two days.\n\n")
	var successCount, failCount, skipCount int
	for i, item := range dataAll {
		fmt.Printf(" %02d. ID: %s\n     Series: %s\n     Episode: S%02dE%02d - %s\n",
			i+1, item.ID, item.SeriesName, item.SeasonNo, item.EpisodeNo, item.Name)

		if isItemFine(client, &config, &item) {
			fmt.Printf("     All desired criteria are met. Skipping.\n\n")
			skipCount++
			continue
		} else {
			fmt.Println("     Some desired criteria are not met. Requesting a refresh...")
		}

		err := refreshItem(client, &config, &item)
		if err == nil {
			successCount++
		} else {
			if err.Error() != "No new data." {
				fmt.Println("     Retrying in 2 seconds...")
				time.Sleep(2 * time.Second)
				err = refreshItem(client, &config, &item)
			}
			if err == nil {
				successCount++
			} else {
				failCount++
				fmt.Printf("     Better luck next time!\n\n")
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
