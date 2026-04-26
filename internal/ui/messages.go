package ui

import "sshm/internal/domain"

type connectionsLoadedMsg struct {
	items []domain.Connection
	err   error
}

type groupsLoadedMsg struct {
	items []domain.ConnectionGroupListItem
	err   error
}

type groupOpDoneMsg struct {
	status            string
	success           bool
	err               error
	reloadGroups      bool
	reloadConnections bool
	groupID           int64
	groupName         string
	clearGroupFilter  bool
}

type importPreviewMsg struct {
	preview domain.ImportPreview
	err     error
}

type importDoneMsg struct {
	summary           domain.ImportSummary
	err               error
	reloadConnections bool
	setScope          bool
	scope             domain.ConnectionListScope
	groupID           int64
	groupName         string
}

type localLoadedMsg struct {
	items      []domain.FileEntry
	path       string
	selectName string
	err        error
}

type remoteLoadedMsg struct {
	items      []domain.FileEntry
	path       string
	selectName string
	err        error
}

type shellReadyMsg struct {
	connectionID int64
}

type opDoneMsg struct {
	status            string
	success           bool
	err               error
	reloadBrowser     bool
	reloadConnections bool
	targetPanel       domain.FilePanel
	selectName        string
}

type transferProgressMsg struct {
	progress domain.TransferProgress
	action   string
	source   <-chan transferProgressMsg
}
