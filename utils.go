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

func callRequest(client *http.Client, cfg *config, t string, path string, params *url.Values) ([]byte, error) {
	req, err := http.NewRequest(t, cfg.URL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+cfg.APIKey+`"`)
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

func fetchItems(client *http.Client, cfg *config, params *url.Values) []item {
	body, err := callRequest(client, cfg, "GET", "/Items", params)
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
		if *item == FineItem {
			return BadImage
		} else {
			return BadAll
		}
	}

	itemStatus := FineItem
	if hasGenericTitle(item.Name) {
		log.Println(Red + "     Title looks like a generic placeholder." + Reset)
		itemStatus = BadTitle
	}
	if strings.TrimSpace(item.Overview) == "" {
		log.Println(Red + "     Overview is missing." + Reset)
		itemStatus = BadOverview
	}

	if body, err := callRequest(client, config, "GET", fmt.Sprintf("/Items/%s/Images", item.ID), nil); err == nil {
		var images []itemImage
		json.Unmarshal(body, &images)
		if len(images) <= 0 {
			log.Println(Red + "     Primary image is missing." + Reset)
			itemStatus = BadImage
		}
		for _, image := range images {
			if image.Type == "Primary" {
				if image.Height < config.DesiredImageHeight {
					log.Printf(Red+"     Primary image is of low resolution (%dx%d)."+Reset, image.Width, image.Height)
					itemStatus = escalateBadness(&itemStatus)
				} else if image.Size < 100*image.Height {
					log.Printf(Red+"     Primary image is too small (%.1f KiB)."+Reset, (float64)(image.Size)/1024)
					itemStatus = escalateBadness(&itemStatus)
				} else {
					break
				}
			}
		}
	} else {
		log.Println(Red + "     Primary image is missing." + Reset)
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

	best := &images[0]
	bestHeight := best.Height

	var tmdbImages []*remoteImage
	for _, image := range images {
		if image.Height > bestHeight {
			best = &image
			bestHeight = image.Height
		}
		if image.ProviderName == "TheMovieDb" {
			tmdbImages = append(tmdbImages, &image)
		}
	}

	if bestHeight == 0 {
		if len(tmdbImages) > 0 {
			return tmdbImages[rand.Intn(len(tmdbImages))]
		}
		return &images[rand.Intn(len(images))]
	}

	return best
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

	if slices.Contains([]badItem{BadTitle, BadOverview, BadAll}, itemStatus) {
		updateParams := url.Values{}
		updateParams.Add("metadataRefreshMode", "FullRefresh")
		updateParams.Add("replaceAllMetadata", "true")

		_, err := callRequest(client, config, "POST", fmt.Sprintf("/Items/%s/Refresh", item.ID), &updateParams)
		if err != nil {
			log.Println(Red+"  Refresh failed:", err, Reset)
			errState = err
		}
	}

	if slices.Contains([]badItem{BadImage, BadAll}, itemStatus) {
		images, err := getRemoteImages(client, config, item)
		if err != nil {
			errState = err
		} else {
			best := getBestImage(images)

			if best == nil {
				log.Println(Red + "     No remote images found." + Reset)
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
		if isItemFine(client, config, &updatedItem) == FineItem {
			fmt.Println("     Refresh successful!")
			fmt.Println(Green + "     The episode now satisfies all the desired criteria.\n" + Reset)
			return nil
		} else {
			fmt.Println(Red + "     The desired criteria are still not met." + Reset)
			return errors.New("No new data.")
		}
	} else {
		fmt.Println(Red+"     Refresh failed:", errState, Reset)
		return errState
	}
}
