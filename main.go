// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"
)

var Version = "unknown"

type Config struct {
	APIKey             string `json:"apiKey"`
	URL                string `json:"jellyfinURL"`
	DesiredImageHeight uint16 `json:"desiredImageHeight"`
	DaysToScan         uint8  `json:"daysToScan"`
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
	Width  uint16 `json:"Width"`
}

type ItemsResponse struct {
	Items []Item `json:"Items"`
}

const (
	Reset = "\033[0m"
	Red   = "\033[31m"
	Green = "\033[32m"
	Blue  = "\033[34m"
)

func main() {
	if len(os.Args) > 1 && slices.Contains([]string{"--version", "-V"}, os.Args[1]) {
		fmt.Println(Version)
		return
	}

	log.SetFlags(0)
	config := loadConfig()

	client := &http.Client{}
	// Get all items released in the last n days
	queryParams := url.Values{}
	queryParams.Add("includeItemTypes", "Episode")
	queryParams.Add("recursive", "true")
	queryParams.Add("fields", "Overview")
	cutoffDate := time.Now().AddDate(0, 0, -int(config.DaysToScan+1)).Format(time.RFC3339)
	queryParams.Add("minPremiereDate", cutoffDate)
	dataAll := fetchItems(client, &config, &queryParams)

	fmt.Println(Blue + "Jellyfin Autorefresh New Releases (SinTan1729)\n----------" + Reset)
	fmt.Println(Blue+"Starting at", time.Now().Format(time.RFC1123)+Reset)
	fmt.Println(Blue+"Connecting to", config.URL+Reset)
	fmt.Printf(Blue+"Processing all episodes released in the last %d days.\n\n"+Reset, config.DaysToScan)
	var successCount, failCount, skipCount int
	for i, item := range dataAll {
		fmt.Printf(" %02d. ID: %s\n     Series: %s\n     Episode: S%02dE%02d - %s\n",
			i+1, item.ID, item.SeriesName, item.SeasonNo, item.EpisodeNo, item.Name)

		itemStatus := isItemFine(client, &config, &item)
		if itemStatus == FineItem {
			fmt.Printf(Green + "     All desired criteria are met. Skipping.\n\n" + Reset)
			skipCount++
			continue
		} else {
			fmt.Println(Red + "     Some desired criteria are not met. Requesting a refresh..." + Reset)
		}

		err := refreshItem(client, &config, &item, itemStatus)
		if err == nil {
			successCount++
		} else {
			if err.Error() != "No new data." {
				fmt.Println("     Retrying in 2 seconds...")
				time.Sleep(2 * time.Second)
				err = refreshItem(client, &config, &item, itemStatus)
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
	fmt.Println(Blue + "Summary:" + Reset)
	fmt.Println(Blue+"  Skipped:", skipCount, Reset)
	fmt.Println(Blue+"  Successful refreshes:", successCount, Reset)
	fmt.Println(Blue+"  Failed refreshes:", failCount, Reset)
	fmt.Printf(Blue + "----------\n\n")
}
