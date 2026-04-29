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
	overlayCommandPalette
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
	browserInputMkdir
	browserInputRename
)

type confirmAction int

const (
	confirmActionNone confirmAction = iota
	confirmActionDeleteConnection
	confirmActionDeleteGroup
	confirmActionBrowserOverwrite
	confirmActionBrowserDelete
)

type AppResult struct {
	ShellSession app.ShellSession
}

type shellState struct {
	width  int
	height int

	page    page
	overlay overlayKind

	status        string
	statusSuccess bool
	err           error
}

type homeScreenState struct {
	searchInput textinput.Model
	search      string
	searchMode  bool
	listScope   domain.ConnectionListScope
	listGroupID int64
	listGroup   string

	connections []Connection
	selected    int
	connecting  bool
}

type screenState struct {
	home    homeScreenState
	form    formState
	browser browserState
	imports importState
}

type overlayState struct {
	groups  groupPanelState
	confirm confirmState
	palette commandPaletteState
}

type commandPaletteState struct {
	input    textinput.Model
	selected int
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
	// 文件工作区只持有文件会话，避免重新依赖交互 shell 能力。
	session app.FileSession

	localPanel  filePanel
	remotePanel filePanel
	activePanel domain.FilePanel

	inputMode browserInputMode
	input     textinput.Model
	inputItem domain.FileEntry
	// pending 只保存“已经完成存在性判断、等待用户最终确认”的操作。
	// 这样 confirm 只负责确认交互，真正的执行与刷新策略仍由 workflow 统一编排。
	pending *browserPendingOperation
}

type browserPendingOperation struct {
	action   string
	run      func(func(domain.TransferProgress)) error
	success  string
	cancel   string
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
	request int
}

type confirmState struct {
	action           confirmAction
	title            string
	description      string
	sourcePath       string
	targetPath       string
	confirmSelection bool
	choiceEnabled    bool
	connectionID     int64
	groupID          int64
	clearGroupFilter bool
}
