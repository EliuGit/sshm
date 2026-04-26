package ui

import (
	"sshm/internal/app"
	"sshm/internal/domain"

	"github.com/charmbracelet/bubbles/textinput"
)

type Connection = domain.Connection

type page int

const (
	pageHome page = iota
	pageForm
	pageBrowser
	pageImport
)

type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayDelete
	overlayHelp
	overlayGroup
	overlayBrowserInput
	overlayBrowserConfirm
)

type groupPanelMode int

const (
	groupPanelFilter groupPanelMode = iota
	groupPanelMove
)

type groupInputMode int

const (
	groupInputNone groupInputMode = iota
	groupInputCreate
	groupInputRename
)

type importStep int

const (
	importStepPath importStep = iota
	importStepPreview
)

type browserInputMode int

const (
	browserInputGoto browserInputMode = iota
	browserInputFilter
)

type AppResult struct {
	ShellSession app.RemoteSession
}

type formState struct {
	editingID        int64
	groupID          *int64
	focusIndex       int
	originalAuthType domain.AuthType
	name             textinput.Model
	host             textinput.Model
	port             textinput.Model
	username         textinput.Model
	description      textinput.Model
	password         textinput.Model
	privateKeyPath   textinput.Model
	authType         domain.AuthType
	keepPassword     bool
	errorMessage     string
	successMessage   string
}

type groupPanelState struct {
	mode       groupPanelMode
	inputMode  groupInputMode
	confirming bool
	items      []domain.ConnectionGroupListItem
	selected   int
	targetID   int64
	input      textinput.Model
	errorValue string
}

type importState struct {
	step      importStep
	path      textinput.Model
	items     []domain.ImportCandidate
	warnings  []string
	selected  int
	loading   bool
	errorText string
}

type browserState struct {
	connectionID int64
	connection   domain.Connection
	session      app.RemoteSession

	localPanel  filePanel
	remotePanel filePanel
	activePanel domain.FilePanel

	inputMode    browserInputMode
	input        textinput.Model
	confirmTitle string
	confirmPath  string
	pending      *browserTransfer
	confirmYes   bool
}

type browserTransfer struct {
	action   string
	source   string
	target   string
	run      func(func(domain.TransferProgress)) error
	success  string
	panel    domain.FilePanel
	selectBy string
}

type filePanel struct {
	panel   domain.FilePanel
	title   string
	path    string
	filter  string
	items   []domain.FileEntry
	cursor  int
	loading bool
}
