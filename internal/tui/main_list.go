package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"sshm/internal/i18n"
	"sshm/internal/repository"
)

// connectionRow 保存连接列表和详情面板需要的展示字段。
type connectionRow struct {
	id                                                           int64
	name, host, port, username, auth, credName, lastUsed, remark string
	credID, useCount                                             int64
}

func rowFromConnection(c repository.Connection) connectionRow {
	lastUsed := ""
	if c.LastUsedAt.Valid {
		lastUsed = time.Unix(c.LastUsedAt.Int64, 0).Format("2006-01-02 15:04")
	}
	return connectionRow{id: c.ID, name: c.Name, host: c.Host, port: fmt.Sprint(c.Port), username: c.Username, auth: authLabel(c.Credential), credID: c.CredentialID, credName: c.CredentialName, lastUsed: lastUsed, remark: c.Remark, useCount: c.UseCount}
}

func authLabel(auth string) string {
	if auth == "passwd" || auth == "密码" {
		return i18n.T("Password")
	}
	if auth == "key" || auth == "私钥" {
		return i18n.T("Private key")
	}
	return auth
}

func authCode(auth string) string {
	if auth == "密码" || auth == i18n.T("Password") {
		return "passwd"
	}
	if auth == "私钥" || auth == i18n.T("Private key") {
		return "key"
	}
	return auth
}

// visibleConnections 按当前搜索词过滤，并仅对当前选中的字段做稳定排序。
func (m model) visibleConnections() []connectionRow {
	connections := m.connections
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	filtered := make([]connectionRow, 0, len(connections))
	for _, c := range connections {
		if query == "" || strings.Contains(strings.ToLower(c.name), query) || strings.Contains(strings.ToLower(c.host), query) || strings.Contains(strings.ToLower(c.username), query) || strings.Contains(strings.ToLower(c.remark), query) {
			filtered = append(filtered, c)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if m.sortField == 0 {
			return filtered[i].useCount > filtered[j].useCount
		}
		var left, right string
		switch m.sortField {
		case 'h':
			left, right = filtered[i].host, filtered[j].host
		case 'u':
			left, right = filtered[i].username, filtered[j].username
		default:
			left, right = filtered[i].name, filtered[j].name
		}
		left, right = strings.ToLower(left), strings.ToLower(right)
		if left == right {
			return filtered[i].useCount > filtered[j].useCount
		}
		if m.sortAsc {
			return left < right
		}
		return left > right
	})
	return filtered
}
