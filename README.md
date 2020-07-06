# extractbdays
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A hacky, messy - but stable - tool written to easily extract animal crossing birthdays from the wiki and add them to a Google Calendar.

This was used to create the birthdays part of [my Animal Crossing Calendar](https://github.com/zedseven/ac-calendar).

This is mostly uploaded as an example implementation of the [Google Calendar API](https://developers.google.com/calendar/) and [Google Drive API](https://developers.google.com/drive/) in Go, though it certainly isn't the cleanest.

## Links I Found Useful
* [A reference of changes between v2 and v3 of the Google Drive API](https://developers.google.com/drive/api/v2/v2-to-v3-reference) - I found this useful to figure out that [the FileUrl field of calendar](https://godoc.org/google.golang.org/api/calendar/v3#EventAttachment.FileUrl) corresponds to [the WebViewUrl field in drive](https://godoc.org/google.golang.org/api/drive/v3#File.WebViewLink)
