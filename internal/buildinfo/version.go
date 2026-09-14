// Package buildinfo 提供构建时注入的应用信息。
package buildinfo

// Version 是应用版本号，可在发布构建时通过 -ldflags -X 注入。
var Version = "dev"
