// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"
)

var Version = "unknown"

func main() {
	if len(os.Args) > 1 && slices.Contains([]string{"--version", "-V"}, os.Args[1]) {
		fmt.Println(Version)
		return
	}

	config := loadConfig()

	client := &http.Client{}
	// Get all items released in the last n days
	queryParams := url.Values{}
	queryParams.Add("includeItemTypes", "Episode")
	queryParams.Add("recursive", "true")
	queryParams.Add("fields", "Overview")
	cutoffDate := time.Now().AddDate(0, 0, -int(config.DaysToScan+1)).UTC().Format(time.RFC3339)
	queryParams.Add("minPremiereDate", cutoffDate)
	dataAll := fetchItems(client, &config, &queryParams)

	printLog(blue, base, "Jellyfin Autorefresh New Releases v%s", Version)
	printLog(blue, base, "https://github.com/SinTan1729/jellyfin-autorefresh-new-releases\n----------")
	printLog(blue, base, "Starting at %s", time.Now().Format(time.RFC1123))
	printLog(blue, base, "Connecting to %s", config.URL)
	printLog(blue, base, "Processing all episodes released in the last %d days.\n", config.DaysToScan)

	pad := level(math.Floor(math.Log10(float64(len(dataAll)))) + 1)
	details += pad
	var successCount, failCount, skipCount int
	for i, item := range dataAll {
		printLog(none, header, "%0*d. ID: %s", pad, i+1, item.ID)
		printLog(none, details, "Series: %s", item.SeriesName)
		printLog(none, details, "Episode: S%02dE%02d - %s", item.SeasonNo, item.EpisodeNo, item.Name)
		if item.PremiereDate != nil {
			printLog(none, details, "Release Date: %s", item.PremiereDate.Local().Format("Monday, Jan 2"))
		}

		itemStatus := isItemFine(client, &config, &item)
		if itemStatus == fineItem {
			printLog(green, details, "All desired criteria are met. Skipping.\n")
			skipCount++
			continue
		} else {
			printLog(red, details, "Some desired criteria are not met. Requesting a refresh...")
		}

		err := refreshItem(client, &config, &item, itemStatus)
		if err == nil {
			successCount++
		} else {
			failCount++
			printLog(none, details, "Better luck next time!\n")
		}
	}
	// Print a summary
	printLog(blue, base, "Summary:")
	printLog(blue, summary, "Skipped: %d", skipCount)
	printLog(blue, summary, "Successful refreshes: %d", successCount)
	printLog(blue, summary, "Failed refreshes: %d", failCount)
}
