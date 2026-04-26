package buildinfo

import "strings"

var Version = "dev"

const Author = "nullecho"

type Metadata struct {
	Version string
	Author  string
}

func Info() Metadata {
	version := strings.TrimSpace(Version)
	if version == "" {
		version = "dev"
	}
	return Metadata{Version: version, Author: Author}
}
