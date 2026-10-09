// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
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
		if Version != "unknown" {
			printLog(none, base, nil, "Jellyfin Autorefresh New Releases v%s", Version)
		} else {
			printLog(none, base, nil, "Jellyfin Autorefresh New Releases (dev)")
		}
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

	if Version == "unknown" {
		printLog(blue, base, nil, "Jellyfin Autorefresh New Releases (dev)")
	} else {
		printLog(blue, base, nil, "Jellyfin Autorefresh New Releases v%s", Version)
	}
	printLog(blue, base, nil, "https://github.com/SinTan1729/jellyfin-autorefresh-new-releases\n----------")
	printLog(blue, base, nil, "Starting at %s", time.Now().Format(time.RFC1123))
	printLog(blue, base, nil, "Connecting to %s", config.URL)
	printLog(blue, base, nil, "Processing all episodes released in the last %d days.\n", config.DaysToScan)

	pad := level(math.Floor(math.Log10(float64(len(dataAll)))) + 1)
	details += pad
	var successCount, failCount, skipCount int
	for i, item := range dataAll {
		var episodeLink, seriesLink string
		if config.HyperlinkDomain != "" {
			episodeLink = createLink(config.HyperlinkDomain, item.EpisodeID)
			seriesLink = createLink(config.HyperlinkDomain, item.SeriesID)
		}
		printLog(none, header, nil, "%0*d. ID: %s", pad, i+1, item.EpisodeID)
		printLog(none, details, &seriesLink, "Series: %s", item.SeriesName)
		printLog(none, details, &episodeLink, "Episode: S%02dE%02d - %s", item.SeasonNo, item.EpisodeNo, item.Name)
		if item.PremiereDate != nil {
			printLog(none, details, nil, "Release Date: %s", item.PremiereDate.Local().Format("Monday, Jan 2"))
		}

		itemStatus := isItemFine(client, &config, &item)
		if itemStatus == fineItem {
			printLog(green, details, nil, "All desired criteria are met. Skipping.\n")
			skipCount++
			continue
		} else {
			printLog(red, details, nil, "Some desired criteria are not met.")
			printLog(none, details, nil, "Requesting a refresh...")
		}

		err := refreshItem(client, &config, &item, itemStatus)
		if err == nil {
			successCount++
		} else {
			failCount++
			printLog(none, details, nil, "Better luck next time!\n")
		}
	}
	// Print a summary
	printLog(blue, base, nil, "Summary:")
	printLog(blue, summary, nil, "Skipped: %d", skipCount)
	printLog(blue, summary, nil, "Successful refreshes: %d", successCount)
	printLog(blue, summary, nil, "Failed refreshes: %d", failCount)
}
