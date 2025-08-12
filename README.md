<!-- SPDX-FileCopyrightText: 2025 Sayantan Santra <sayantan.santra689@gmail.com> -->
<!-- SPDX-License-Identifier: GPL-3.0-only -->

[![maintainer-badge](https://img.shields.io/badge/maintainer-SinTan1729-blue)](https://github.com/SinTan1729)
[![latest-release-badge](https://img.shields.io/github/v/release/SinTan1729/jellyfin-autorefresh-new-releases?label=latest%20release)](https://github.com/SinTan1729/jellyfin-autorefresh-new-releases/releases/latest)
[![license-badge](https://img.shields.io/github/license/SinTan1729/jellyfin-autorefresh-new-releases)](https://spdx.org/licenses/GPL-3.0-only.html)

# Jellyfin Autorefresh New Releases

This is a simple Go application to request refreshes for newly released items in Jellyfin, where some
info is missing. Refreshes are requested if image or overview is missing in an episode. It only works
with episodes released in the last two days. It's mainly meant to work for episodes where TMDB doesn't
have the information on release.

# Installation

## Installation from source

1. Clone the repo.

```
git clone https://github.com/SinTan1729/jellyfin-autorefresh-new-releases
```

2. Install.

```
cd jellyfin-autorefresh-new-releases
make install
```

3. You can uninstall by running `make uninstall`.

## Installation from AUR

Use the AUR package [`jellyfin-autorefresh-new-releases-bin`](https://aur.archlinux.org/packages/jellyfin-autorefresh-new-releases-bin).

## Installation from LURE

This should (at least in theory) work for every distro, and should be similar to AUR in terms of experience.

1. Install `LURE` from [lure.sh](https://lure.sh).
2. Add my personal repo to it.

```
lure addrepo -n SinTan1729 -u https://github.com/SinTan1729/lure-repo
```

3. Install `jellyfin-autorefresh-new-releases`

```
lure in jellyfin-autorefresh-new-releases
```

# Usage

Config will be loaded from `$XDG_CONFIG_HOME/jellyfin-autorefresh-new-releases/config.json` and should look like
the following.

```json
{
  "BaseURI": "<jellyfin-instance-uri>",
  "Key": "<api-key>"
}
```

It's recommended that you use a local/internal URI for better performance.

With proper config in place, just run `jellyfin-autorefresh`. You may want to run it periodically using a cronjob or equivalent.

# Notes

- I haven't used any external packages, everything is written in pure Go, using the Go Standard Library. I'll try to keep it this way.
