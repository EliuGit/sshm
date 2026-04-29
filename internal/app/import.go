package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sshm/internal/domain"
	"strconv"
	"strings"
)

type ImportService struct {
	repo                  Repository
	connections           *ConnectionService
	defaultPrivateKeyPath string
}

type sshHostBlock struct {
	Aliases      []string
	HostName     string
	User         string
	Port         int
	IdentityFile string
	GroupName    string
	SourcePath   string
	Warnings     []string
	Unsupported  []string
}

func (s *ImportService) PreviewSSHConfig(path string) (domain.ImportPreview, error) {
	blocks, warnings, err := parseSSHConfig(path)
	if err != nil {
		return domain.ImportPreview{}, err
	}
	existing, err := s.repo.ListConnections(domain.ConnectionListOptions{})
	if err != nil {
		return domain.ImportPreview{}, err
	}
	candidates := make([]domain.ImportCandidate, 0, len(blocks))
	for _, block := range blocks {
		for _, alias := range block.Aliases {
			if hasHostPattern(alias) {
				candidates = append(candidates, domain.ImportCandidate{
					Connection: domain.ConnectionInput{Name: alias},
					Skipped:    true,
					Action:     domain.ImportActionSkip,
					Warnings:   []string{warningSkippedPatternHost()},
				})
				continue
			}
			host := strings.TrimSpace(block.HostName)
			if host == "" {
				host = alias
			}
			username := strings.TrimSpace(block.User)
			if username == "" {
				username = currentUsername()
			}
			port := block.Port
			if port == 0 {
				port = 22
			}
			keyPath := strings.TrimSpace(block.IdentityFile)
			if keyPath == "" {
				keyPath = s.defaultPrivateKeyPath
			}
			warns := append([]string{}, block.Warnings...)
			for _, item := range block.Unsupported {
				warns = append(warns, warningUnsupportedDirective(item))
			}
			input := domain.ConnectionInput{
				Name:           alias,
				Host:           host,
				Port:           port,
				Username:       username,
				AuthType:       domain.AuthTypePrivateKey,
				PrivateKeyPath: keyPath,
				Description:    fmt.Sprintf("Imported from %s", filepath.Base(block.SourcePath)),
			}
			candidate := domain.ImportCandidate{
				Connection: input,
				GroupName:  strings.TrimSpace(block.GroupName),
				Warnings:   warns,
				Action:     domain.ImportActionCreate,
			}
			if matched := findImportConflict(existing, input); matched != nil {
				candidate.ExistingID = matched.ID
				candidate.Action = domain.ImportActionSkip
			}
			candidates = append(candidates, candidate)
		}
	}
	return domain.ImportPreview{Candidates: candidates, Warnings: warnings}, nil
}

func (s *ImportService) Apply(preview domain.ImportPreview) (domain.ImportSummary, error) {
	var summary domain.ImportSummary
	for _, candidate := range preview.Candidates {
		if candidate.Skipped || candidate.Action == domain.ImportActionSkip {
			summary.Skipped++
			continue
		}
		groupID, err := s.resolveImportGroup(candidate.GroupName)
		if err != nil {
			return summary, err
		}
		input := candidate.Connection
		input.GroupID = groupID
		switch candidate.Action {
		case domain.ImportActionUpdate:
			if candidate.ExistingID == 0 {
				summary.Skipped++
				continue
			}
			if _, err := s.connections.Update(candidate.ExistingID, domain.ConnectionUpdateInput{
				GroupID:        input.GroupID,
				Name:           input.Name,
				Host:           input.Host,
				Port:           input.Port,
				Username:       input.Username,
				AuthType:       input.AuthType,
				PrivateKeyPath: input.PrivateKeyPath,
				Description:    input.Description,
				KeepPassword:   false,
				Password:       input.Password,
			}); err != nil {
				return summary, err
			}
			summary.Updated++
		case domain.ImportActionCopy:
			input.Name = uniqueCopyName(input.Name)
			if _, err := s.connections.Create(input); err != nil {
				return summary, err
			}
			summary.Created++
		default:
			if _, err := s.connections.Create(input); err != nil {
				return summary, err
			}
			summary.Created++
		}
	}
	return summary, nil
}

func (s *ImportService) resolveImportGroup(name string) (*int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	group, found, err := s.repo.FindGroupByName(name)
	if err != nil {
		return nil, err
	}
	if !found {
		group, err = s.repo.CreateGroup(name)
		if err != nil {
			return nil, err
		}
	}
	return &group.ID, nil
}

func parseSSHConfig(path string) ([]sshHostBlock, []string, error) {
	visited := map[string]bool{}
	blocks, warnings, err := parseSSHConfigFile(expandPath(path), "", visited)
	if err != nil {
		return nil, nil, err
	}
	return blocks, warnings, nil
}

func parseSSHConfigFile(path string, inheritedGroup string, visited map[string]bool) ([]sshHostBlock, []string, error) {
	cleanPath := filepath.Clean(path)
	if visited[cleanPath] {
		return nil, nil, nil
	}
	visited[cleanPath] = true

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var blocks []sshHostBlock
	var warnings []string
	currentGroup := inheritedGroup
	var current *sshHostBlock
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "#") {
			if group, ok := parseGroupComment(raw); ok {
				currentGroup = group
			}
			continue
		}
		line := stripSSHComment(raw)
		if line == "" {
			continue
		}
		key, value := splitSSHDirective(line)
		keyLower := strings.ToLower(key)
		switch keyLower {
		case "include":
			for _, includeValue := range strings.Fields(value) {
				includePath := resolveSSHInclude(cleanPath, includeValue)
				matches, err := filepath.Glob(includePath)
				if err != nil || len(matches) == 0 {
					matches = []string{includePath}
				}
				for _, match := range matches {
					included, includeWarnings, err := parseSSHConfigFile(match, currentGroup, visited)
					if err != nil {
						warnings = append(warnings, warningIncludeReadFailed(match, err))
						continue
					}
					blocks = append(blocks, included...)
					warnings = append(warnings, includeWarnings...)
				}
			}
		case "host":
			if current != nil {
				blocks = append(blocks, *current)
			}
			aliases := strings.Fields(value)
			current = &sshHostBlock{
				Aliases:    aliases,
				GroupName:  currentGroup,
				SourcePath: cleanPath,
			}
		default:
			if current == nil {
				continue
			}
			applySSHDirective(current, keyLower, value)
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return blocks, warnings, nil
}

func applySSHDirective(block *sshHostBlock, key string, value string) {
	value = strings.TrimSpace(value)
	switch key {
	case "hostname":
		block.HostName = firstSSHValue(value)
	case "user":
		block.User = firstSSHValue(value)
	case "port":
		port, err := strconv.Atoi(firstSSHValue(value))
		if err != nil || port <= 0 || port > 65535 {
			block.Warnings = append(block.Warnings, warningInvalidPortFallback())
			return
		}
		block.Port = port
	case "identityfile":
		block.IdentityFile = firstSSHValue(value)
	case "proxyjump", "proxycommand", "match":
		block.Unsupported = append(block.Unsupported, key)
	}
}

func splitSSHDirective(line string) (string, string) {
	key, value, ok := strings.Cut(line, " ")
	if !ok {
		key, value, _ = strings.Cut(line, "\t")
	}
	return strings.TrimSpace(key), strings.TrimSpace(value)
}

func firstSSHValue(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], `"`)
}

func stripSSHComment(line string) string {
	inQuote := false
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == '"' {
			inQuote = !inQuote
			continue
		}
		if char == '#' && !inQuote {
			return strings.TrimSpace(line[:index])
		}
	}
	return line
}

func parseGroupComment(line string) (string, bool) {
	value := strings.TrimSpace(strings.TrimPrefix(line, "#"))
	if !strings.HasPrefix(strings.ToLower(value), "sshm:group=") {
		return "", false
	}
	_, group, _ := strings.Cut(value, "=")
	return strings.TrimSpace(group), true
}

func resolveSSHInclude(basePath string, value string) string {
	value = expandPath(strings.Trim(value, `"`))
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(filepath.Dir(basePath), value)
}

func warningSkippedPatternHost() string {
	return "通配 Host 已跳过"
}

func warningUnsupportedDirective(item string) string {
	return fmt.Sprintf("暂不支持 %s，已忽略", item)
}

func warningIncludeReadFailed(path string, err error) string {
	return fmt.Sprintf("Include %s 读取失败：%v", path, err)
}

func warningInvalidPortFallback() string {
	return "端口无效，已使用 22"
}

func hasHostPattern(value string) bool {
	return strings.ContainsAny(value, "*?!")
}

func currentUsername() string {
	if value := strings.TrimSpace(os.Getenv("USER")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("USERNAME")); value != "" {
		return value
	}
	return "root"
}

func findImportConflict(existing []domain.Connection, input domain.ConnectionInput) *domain.Connection {
	inputName := strings.ToLower(strings.TrimSpace(input.Name))
	inputHost := strings.ToLower(strings.TrimSpace(input.Host))
	inputUser := strings.ToLower(strings.TrimSpace(input.Username))
	for index := range existing {
		conn := existing[index]
		if strings.ToLower(conn.Name) == inputName {
			return &conn
		}
		if strings.ToLower(conn.Host) == inputHost && strings.ToLower(conn.Username) == inputUser && conn.Port == input.Port {
			return &conn
		}
	}
	return nil
}

func uniqueCopyName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Imported Copy"
	}
	return name + " Copy"
}
