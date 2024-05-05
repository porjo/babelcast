package main

import (
	"embed"
	"io/fs"
	"log"
)

// embedContent holds our static web server content.
// embedContentHtml moves the filesystem root to html/
//
//go:embed html
var embedContent embed.FS
var embedContentHtml fs.FS

func init() {
	var err error
	embedContentHtml, err = fs.Sub(embedContent, "html")
	if err != nil {
		log.Fatal(err)
	}
}
