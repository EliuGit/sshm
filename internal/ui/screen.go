package ui

import tea "github.com/charmbracelet/bubbletea"

// screen 只负责页面级输入和渲染，Model 负责全局消息分发。
type screen interface {
	update(*Model, tea.Msg) (tea.Model, tea.Cmd)
	view(*Model) string
}

type homeScreen struct{}

func (homeScreen) update(model *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return model.updateHome(msg)
}
func (homeScreen) view(model *Model) string { return model.viewHome() }

type formScreen struct{}

func (formScreen) update(model *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return model.updateForm(msg)
}
func (formScreen) view(model *Model) string {
	return model.styles.App.Render(model.viewForm())
}

type browserScreen struct{}

func (browserScreen) update(model *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return model.updateBrowser(msg)
}
func (browserScreen) view(model *Model) string { return model.viewBrowser() }

type importScreen struct{}

func (importScreen) update(model *Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return model.updateImport(msg)
}
func (importScreen) view(model *Model) string {
	return model.styles.App.Render(model.viewImport())
}

func (m *Model) currentScreen() screen {
	switch m.page {
	case pageForm:
		return formScreen{}
	case pageBrowser:
		return browserScreen{}
	case pageImport:
		return importScreen{}
	default:
		return homeScreen{}
	}
}
