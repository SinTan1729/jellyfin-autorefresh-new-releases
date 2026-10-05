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
	SeasonNo     uint32     `json:"ParentIndexNumber"`
	EpisodeNo    uint32     `json:"IndexNumber"`
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

type color string

const (
	none  color = "\033[0m"
	red   color = "\033[31m"
	green color = "\033[32m"
	blue  color = "\033[34m"
)

type level int

var details level = 3

const (
	base    level = 0
	header  level = 1
	summary level = 2
)

type badItem int

const (
	fineItem    badItem = 0
	badOverview badItem = 1
	badTitle    badItem = 2
	badImage    badItem = 3
	badAll      badItem = 4
)

type remoteImage struct {
	ProviderName string `json:"ProviderName"`
	Url          string `json:"Url"`
	Width        uint32 `json:"Width"`
	Height       uint32 `json:"Height"`
	VoteCount    uint32 `json:"VoteCount"`
	Type         string `json:"Type"`
	sortSeed     uint32
}
type remoteImagesResponse struct {
	Images []remoteImage `json:"Images"`
}
