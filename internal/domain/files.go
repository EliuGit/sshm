package domain

import (
	"os"
	"time"
)

type FilePanel int

const (
	LocalPanel FilePanel = iota
	RemotePanel
)

func (p FilePanel) String() string {
	switch p {
	case RemotePanel:
		return "remote"
	default:
		return "local"
	}
}

type FileEntry struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
	Mode    os.FileMode
	IsDir   bool
	Panel   FilePanel
}

type TransferProgress struct {
	Path  string
	Bytes int64
	Total int64
}
