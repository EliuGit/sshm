package ui

import (
	"sshm/internal/domain"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// importWorkflow 负责导入页的业务编排。
//
// 这里统一处理“预览 -> 应用 -> 导入完成后切换列表范围”这条链路，
// 让 updateImport 只保留输入分发，避免后续在页面层继续堆业务规则。
type importWorkflow struct {
	model *Model
}

func (m *Model) importWorkflow() importWorkflow {
	return importWorkflow{model: m}
}

func (w importWorkflow) previewCmd(path string) tea.Cmd {
	m := w.model
	return func() tea.Msg {
		preview, err := m.services.Imports.PreviewSSHConfig(path)
		return importPreviewMsg{preview: preview, err: err}
	}
}

func (w importWorkflow) applyCmd() tea.Cmd {
	m := w.model
	preview := domain.ImportPreview{
		Candidates: append([]domain.ImportCandidate{}, m.imports.items...),
		Warnings:   append([]string{}, m.imports.warnings...),
	}
	return func() tea.Msg {
		summary, err := m.services.Imports.Apply(preview)
		msg := importDoneMsg{summary: summary, err: err, reloadConnections: true}
		if err != nil {
			return msg
		}
		scope, groupID, groupName, setScope := w.resolveAppliedScope(preview)
		msg.setScope = setScope
		msg.scope = scope
		msg.groupID = groupID
		msg.groupName = groupName
		return msg
	}
}

func (w importWorkflow) resolveAppliedScope(preview domain.ImportPreview) (domain.ConnectionListScope, int64, string, bool) {
	m := w.model
	scope := domain.ConnectionListScopeAll
	groupName, ok := singleImportedGroup(preview.Candidates)
	if !ok {
		return scope, 0, "", true
	}
	if groupName == "" {
		return domain.ConnectionListScopeUngrouped, 0, m.translator.T("group.ungrouped"), true
	}
	groups, err := m.services.Groups.List()
	if err != nil {
		return scope, 0, "", true
	}
	for _, group := range groups {
		if !group.Ungrouped && group.Name == groupName {
			return domain.ConnectionListScopeGroup, group.ID, group.Name, true
		}
	}
	return scope, 0, "", true
}

func singleImportedGroup(items []domain.ImportCandidate) (string, bool) {
	groupName := ""
	seen := false
	for _, item := range items {
		if item.Skipped || item.Action == domain.ImportActionSkip {
			continue
		}
		name := strings.TrimSpace(item.GroupName)
		if !seen {
			groupName = name
			seen = true
			continue
		}
		if groupName != name {
			return "", false
		}
	}
	return groupName, seen
}
