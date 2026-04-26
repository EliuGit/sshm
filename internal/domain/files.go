package domain

import "time"

type FilePanel int

const (
	LocalPanel FilePanel = iota
	RemotePanel
)

type FileEntry struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	Panel   FilePanel
}

type TransferProgress struct {
	Path  string
	Bytes int64
	Total int64
}
