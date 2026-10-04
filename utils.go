// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

func printLog(col color, level level, msg string, vars ...any) {
	spaces := strings.Repeat(" ", int(level))
	msg = fmt.Sprintf("%s%s", spaces, msg)
	msg = fmt.Sprintf(msg, vars...)
	if col != none {
		fmt.Printf("%s%s%s\n", col, msg, none)
	} else {
		fmt.Println(msg)
	}
}

func loadConfig() config {
	configDir, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		configDir = "~/.config"
	}
	file, err := os.Open(configDir + "/jellyfin-autorefresh-new-releases/config.json")
	if err != nil {
		log.Fatalln("Could not load config from " + configDir + "/jellyfin-autorefresh-new-releases/config.json. Quitting!")
	}
	defer file.Close()

	config := config{DesiredImageHeight: 360, DaysToScan: 2} //Default value
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

func callRequest(client *http.Client, config *config, t string, path string, params *url.Values) ([]byte, error) {
	req, err := http.NewRequest(t, config.URL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+config.APIKey+`"`)
	if params != nil {
		req.URL.RawQuery = params.Encode()
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if !slices.Contains([]int{200, 204}, resp.StatusCode) {
		return nil, fmt.Errorf("Request failed. Please check the API key. \nError: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func fetchItems(client *http.Client, config *config, params *url.Values) []item {
	body, err := callRequest(client, config, "GET", "/Items", params)
	if err != nil {
		log.Fatalln(err)
	}

	var parsed itemsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Fatalln(err)
	}

	items := parsed.Items
	slices.SortFunc(parsed.Items, func(a, b item) int {
		return a.PremiereDate.Compare(*b.PremiereDate)
	})
	return items
}

func isItemFine(client *http.Client, config *config, item *item) badItem {
	var genericTitlePattern = regexp.MustCompile(`(?i)^\s*(episode|folge|épisode|episodio|epis[oó]dio|aflevering)\s*0*\d+\s*$`)
	hasGenericTitle := func(name string) bool {
		return genericTitlePattern.MatchString(strings.TrimSpace(name))
	}
	escalateBadness := func(item *badItem) badItem {
		if *item == fineItem {
			return badImage
		} else {
			return badAll
		}
	}

	itemStatus := fineItem
	if hasGenericTitle(item.Name) {
		printLog(red, details, "Title looks like a generic placeholder.")
		itemStatus = badTitle
	}
	if strings.TrimSpace(item.Overview) == "" {
		printLog(red, details, "Overview is missing.")
		itemStatus = badOverview
	}

	if body, err := callRequest(client, config, "GET", fmt.Sprintf("/Items/%s/Images", item.ID), nil); err == nil {
		var images []itemImage
		json.Unmarshal(body, &images)
		if len(images) <= 0 {
			printLog(red, details, "Primary image is missing.")
			itemStatus = badImage
		}
		for _, image := range images {
			if image.Type == "Primary" {
				if image.Height < config.DesiredImageHeight {
					printLog(red, details, "Primary image is of low resolution (%dx%d).", image.Width, image.Height)
					itemStatus = escalateBadness(&itemStatus)
				} else if image.Size < 100*image.Height {
					printLog(red, details, "Primary image is too small (%.1f KiB).", (float64)(image.Size)/1024)
					itemStatus = escalateBadness(&itemStatus)
				} else {
					break
				}
			}
		}
	} else {
		printLog(red, details, "Primary image is missing.")
		itemStatus = escalateBadness(&itemStatus)
	}

	return itemStatus
}

func getRemoteImages(
	client *http.Client,
	config *config,
	item *item,
) ([]remoteImage, error) {

	params := url.Values{}
	params.Set("Type", "Primary")
	params.Set("Limit", "100")

	endpoint := fmt.Sprintf(
		"/Items/%s/RemoteImages?%s",
		item.ID,
		params.Encode(),
	)

	body, err := callRequest(client, config, "GET", endpoint, &params)
	if err != nil {
		return nil, err
	}

	var result remoteImagesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Images, nil
}

func getBestImage(images []remoteImage) *remoteImage {
	if len(images) == 0 {
		return nil
	}

	for i := range images {
		images[i].sortSeed = rand.Uint32()
	}

	providerRank := func(i remoteImage) int {
		switch i.ProviderName {
		case "TheMovieDb":
			return 1
		case "TheTVDB":
			return 2
		default:
			return 3
		}
	}

	slices.SortFunc(images, func(a, b remoteImage) int {
		return cmp.Or(
			cmp.Compare(b.Height, a.Height),
			cmp.Compare(providerRank(a), providerRank(b)),
			cmp.Compare(b.VoteCount, a.VoteCount),
			cmp.Compare(a.sortSeed, b.sortSeed),
		)
	})

	return &images[0]
}

func setRemoteImage(
	client *http.Client,
	config *config,
	item *item,
	image *remoteImage,
) error {

	params := url.Values{}
	params.Set("Type", image.Type)
	params.Set("ImageUrl", image.Url)

	endpoint := fmt.Sprintf(
		"/Items/%s/RemoteImages/Download?%s",
		item.ID,
		params.Encode(),
	)

	_, err := callRequest(client, config, "POST", endpoint, &params)
	if err != nil {
		return err
	}

	return nil
}

func refreshItem(client *http.Client, config *config, item *item, itemStatus badItem) error {
	var errState error

	if slices.Contains([]badItem{badTitle, badOverview, badAll}, itemStatus) {
		updateParams := url.Values{}
		updateParams.Add("metadataRefreshMode", "FullRefresh")
		updateParams.Add("replaceAllMetadata", "true")

		_, err := callRequest(client, config, "POST", fmt.Sprintf("/Items/%s/Refresh", item.ID), &updateParams)
		if err != nil {
			printLog(red, details, "  Refresh failed:", err)
			errState = err
		}
	}

	if slices.Contains([]badItem{badImage, badAll}, itemStatus) {
		images, err := getRemoteImages(client, config, item)
		if err != nil {
			errState = err
		} else {
			best := getBestImage(images)

			if best == nil {
				printLog(red, details, "No remote images found.")
				return errors.New("No remote images found")
			}

			if best.Width > 0 && best.Height > 0 {
				printLog(
					none, details,
					"Selected image: %dx%d (%s, Votes: %d)",
					best.Width,
					best.Height,
					best.ProviderName,
					best.VoteCount,
				)
			} else {
				printLog(
					none, details,
					"Selected image: Unknown dimensions (%s, Votes: %d)",
					best.ProviderName,
					best.VoteCount,
				)
			}
			if err := setRemoteImage(client, config, item, best); err != nil {
				errState = err
			}
		}
	}

	if errState == nil {
		// Wait five seconds so that the metadata is actually updated
		time.Sleep(5 * time.Second)
		// Check if the update was successful
		queryParams := url.Values{}
		queryParams.Add("ids", item.ID)
		queryParams.Add("fields", "Overview")
		updatedItem := fetchItems(client, config, &queryParams)[0]
		if isItemFine(client, config, &updatedItem) == fineItem {
			printLog(none, details, "Refresh successful!")
			printLog(green, details, "The episode now satisfies all the desired criteria.\n")
			return nil
		} else {
			printLog(red, details, "The desired criteria are still not met.")
			return errors.New("No new data.")
		}
	} else {
		printLog(red, details, "Refresh failed:", errState)
		return errState
	}
}
