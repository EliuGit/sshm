package tui

import "charm.land/lipgloss/v2"

var (
	// 基础界面颜色。
	backgroundColor   = lipgloss.Color("#282C34")
	textColor         = lipgloss.Color("#D8DEE9")
	mutedTextColor    = lipgloss.Color("#9AA1AE")
	borderColor       = lipgloss.Color("#5C6068")
	accentColor       = lipgloss.Color("#00B3E4")
	selectedColor     = lipgloss.Color("#007EA1")
	selectedTextColor = lipgloss.Color("#FFFFFF")
	labelColor        = lipgloss.Color("#98C379")
	modalBorderColor  = textColor

	// 操作状态颜色。
	warningColor       = lipgloss.Color("#E5C07B")
	errorColor         = lipgloss.Color("#FF6B6B")
	multiSelectedColor = lipgloss.Color("#41454C")

	// 文件传输内容颜色。
	localTagColor  = lipgloss.Color("#1473E6")
	remoteTagColor = lipgloss.Color("#E06C75")
	folderColor    = warningColor
	fileColor      = lipgloss.Color("#61AFEF")
)

// 文件类型图标使用独立调色板，不与界面状态颜色混用。
const (
	transferIconGoColor           = "#6ED8E5"
	transferIconScriptColor       = "#F39C12"
	transferIconTypeScriptColor   = "#2980B9"
	transferIconPythonColor       = "#3498DB"
	transferIconJavaColor         = "#E67E22"
	transferIconCColor            = "#0188D2"
	transferIconCSSColor          = "#2D53E5"
	transferIconJSONColor         = "#F1C40F"
	transferIconTextColor         = "#7F8C8D"
	transferIconPDFColor          = "#D35400"
	transferIconEnvColor          = "#EED645"
	transferIconSQLColor          = "#FF8400"
	transferIconShellColor        = "#2ECC71"
	transferIconImageArchiveColor = "#E74C3C"
	transferIconAudioColor        = "#EE524F"
	transferIconVideoColor        = "#C0392B"
)
