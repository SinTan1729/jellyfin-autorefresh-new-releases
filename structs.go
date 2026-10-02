// SPDX-FileCopyrightText: 2026 Sayantan Santra <sayantan.santra689@gmail.com>
// SPDX-License-Identifier: GPL-3.0-only

package main

import "time"

type config struct {
	APIKey             string `json:"apiKey"`
	URL                string `json:"jellyfinURL"`
	DesiredImageHeight uint32 `json:"desiredImageHeight"`
	DaysToScan         uint8  `json:"daysToScan"`
}

type item struct {
	ID           string     `json:"Id"`
	Name         string     `json:"Name"`
	SeriesName   string     `json:"SeriesName"`
	SeasonNo     uint8      `json:"ParentIndexNumber"`
	EpisodeNo    uint16     `json:"IndexNumber"`
	Overview     string     `json:"Overview"`
	PremiereDate *time.Time `json:"PremiereDate"`
}

type itemImage struct {
	Type   string `json:"ImageType"`
	Height uint32 `json:"Height"`
	Width  uint32 `json:"Width"`
	Size   uint32 `json:"Size"`
}

type itemsResponse struct {
	Items []item `json:"Items"`
}

const (
	Reset = "\033[0m"
	Red   = "\033[31m"
	Green = "\033[32m"
	Blue  = "\033[34m"
)

type badItem int

const (
	FineItem    badItem = 0
	BadOverview badItem = 1
	BadTitle    badItem = 2
	BadImage    badItem = 3
	BadAll      badItem = 4
)

type remoteImage struct {
	ProviderName string `json:"ProviderName"`
	Url          string `json:"Url"`
	Width        uint16 `json:"Width"`
	Height       uint16 `json:"Height"`
	Type         string `json:"Type"`
	sortSeed     uint32
}
type remoteImagesResponse struct {
	Images []remoteImage `json:"Images"`
}
