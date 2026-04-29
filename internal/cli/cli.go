package cli

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sshm/internal/app"
	"sshm/internal/buildinfo"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"
	"text/tabwriter"
)

const (
	exitSuccess = 0
	exitFailure = 1
)

type Command struct {
	services   *app.Services
	translator *i18n.Translator
	stdout     io.Writer
	stderr     io.Writer
}

type taskKind int

const (
	taskHelp taskKind = iota
	taskList
	taskRun
	taskUpload
	taskDownload
	taskVersion
)

type task struct {
	kind     taskKind
	names    []string
	group    string
	filter   string
	command  string
	file     string
	local    string
	remote   string
	force    bool
	failFast bool
}

func New(services *app.Services, translator *i18n.Translator, stdout io.Writer, stderr io.Writer) *Command {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if translator == nil {
		translator, _ = i18n.New("en")
	}
	return &Command{services: services, translator: translator, stdout: stdout, stderr: stderr}
}

func (c *Command) Run(args []string) int {
	parsed, err := parse(args)
	if err != nil {
		fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.cliError(err))
		fmt.Fprintln(c.stderr)
		c.writeHelp(c.stderr)
		return exitFailure
	}
	if parsed.kind == taskHelp {
		c.writeHelp(c.stdout)
		return exitSuccess
	}

	switch parsed.kind {
	case taskList:
		return c.listConnections(parsed)
	case taskRun:
		return c.runCommand(parsed)
	case taskUpload:
		return c.upload(parsed)
	case taskDownload:
		return c.download(parsed)
	case taskVersion:
		fmt.Fprintln(c.stdout, buildinfo.Info().Version)
		return exitSuccess
	default:
		fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.t("cli.err.unknown_command"))
		return exitFailure
	}
}

func (c *Command) listConnections(parsed task) int {
	opts := domain.ConnectionListOptions{Query: parsed.filter}
	if err := c.applyGroupFilter(&opts, parsed.group); err != nil {
		fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.cliError(err))
		return exitFailure
	}

	connections, err := c.services.Connections.ListWithOptions(opts)
	if err != nil {
		fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.cliError(err))
		return exitFailure
	}

	writer := tabwriter.NewWriter(c.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(writer, "%s\t%s\t%s\n", c.t("cli.ls.header_name"), c.t("cli.ls.header_host"), c.t("cli.ls.header_description"))
	for _, conn := range connections {
		fmt.Fprintf(writer, "%s\t%s\t%s\n", conn.Name, conn.Host, conn.Description)
	}
	_ = writer.Flush()
	return exitSuccess
}

func (c *Command) applyGroupFilter(opts *domain.ConnectionListOptions, groupName string) error {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return nil
	}

	if strings.EqualFold(groupName, "ungrouped") || strings.EqualFold(groupName, c.t("group.ungrouped")) {
		opts.Scope = domain.ConnectionListScopeUngrouped
		return nil
	}

	groups, err := c.services.Groups.List()
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.Ungrouped {
			continue
		}
		if strings.EqualFold(group.Name, groupName) {
			opts.Scope = domain.ConnectionListScopeGroup
			opts.GroupID = group.ID
			return nil
		}
	}
	return newCLIError("cli.err.group_not_found", groupName)
}

func (c *Command) runCommand(parsed task) int {
	commandText := parsed.command
	if parsed.file != "" {
		content, err := os.ReadFile(parsed.file)
		if err != nil {
			fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.t("cli.err.read_script_failed", err))
			return exitFailure
		}
		commandText = string(content)
	}

	connections, ok := c.resolveConnections(parsed.names)
	if !ok {
		return exitFailure
	}

	failed := false
	batch := len(connections) > 1
	for _, conn := range connections {
		fmt.Fprintf(c.stderr, c.t("cli.run.start"), conn.Name)
		stdout := c.stdout
		stderr := c.stderr
		if batch {
			stdout = newPrefixWriter(c.stdout, "["+conn.Name+"] ")
			stderr = newPrefixWriter(c.stderr, "["+conn.Name+"] ")
		}
		if err := c.services.Sessions.RunCommand(conn.ID, commandText, stdout, stderr); err != nil {
			failed = true
			fmt.Fprintf(c.stderr, c.t("cli.run.failed"), conn.Name, c.cliError(err))
			if parsed.failFast {
				break
			}
			continue
		}
		fmt.Fprintf(c.stderr, c.t("cli.run.success"), conn.Name)
	}
	return exitCode(failed)
}

func (c *Command) upload(parsed task) int {
	connections, ok := c.resolveConnections(parsed.names)
	if !ok {
		return exitFailure
	}
	localPath := filepath.Clean(parsed.local)
	targetName := filepath.Base(localPath)
	if targetName == "." || targetName == string(filepath.Separator) {
		fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.t("cli.err.invalid_local_path"))
		return exitFailure
	}

	failed := false
	for _, conn := range connections {
		targetPath := joinRemote(parsed.remote, targetName)
		fmt.Fprintf(c.stderr, c.t("cli.upload.start"), conn.Name, localPath, targetPath)
		exists, err := c.services.Files.ExistsRemote(conn.ID, targetPath)
		if err != nil {
			failed = true
			fmt.Fprintf(c.stderr, c.t("cli.upload.check_failed"), conn.Name, c.cliError(err))
			if parsed.failFast {
				break
			}
			continue
		}
		if exists && !parsed.force {
			failed = true
			fmt.Fprintf(c.stderr, c.t("cli.upload.exists"), conn.Name)
			if parsed.failFast {
				break
			}
			continue
		}
		if err := c.services.Files.Upload(conn.ID, localPath, parsed.remote, nil); err != nil {
			failed = true
			fmt.Fprintf(c.stderr, c.t("cli.upload.failed"), conn.Name, c.cliError(err))
			if parsed.failFast {
				break
			}
			continue
		}
		fmt.Fprintf(c.stderr, c.t("cli.upload.success"), conn.Name)
	}
	return exitCode(failed)
}

func (c *Command) download(parsed task) int {
	connections, ok := c.resolveConnections(parsed.names)
	if !ok {
		return exitFailure
	}
	if len(connections) != 1 {
		fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.t("cli.err.download_batch_not_allowed"))
		return exitFailure
	}

	conn := connections[0]
	remotePath := cleanRemote(parsed.remote)
	targetName := path.Base(remotePath)
	if targetName == "." || targetName == "/" {
		fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.t("cli.err.invalid_remote_path"))
		return exitFailure
	}
	localDir := filepath.Clean(parsed.local)
	targetPath := filepath.Join(localDir, targetName)

	fmt.Fprintf(c.stderr, c.t("cli.download.start"), conn.Name, remotePath, targetPath)
	exists, err := c.services.Files.ExistsLocal(targetPath)
	if err != nil {
		fmt.Fprintf(c.stderr, c.t("cli.download.check_failed"), conn.Name, c.cliError(err))
		return exitFailure
	}
	if exists && !parsed.force {
		fmt.Fprintf(c.stderr, c.t("cli.download.exists"), conn.Name)
		return exitFailure
	}
	if err := c.services.Files.Download(conn.ID, remotePath, localDir, nil); err != nil {
		fmt.Fprintf(c.stderr, c.t("cli.download.failed"), conn.Name, c.cliError(err))
		return exitFailure
	}
	fmt.Fprintf(c.stderr, c.t("cli.download.success"), conn.Name)
	return exitSuccess
}

func parse(args []string) (task, error) {
	if len(args) == 0 {
		return task{kind: taskHelp}, nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		return task{kind: taskHelp}, nil
	case "ls":
		return parseList(args[1:])
	case "run":
		return parseRun(args[1:])
	case "upload":
		return parseTransfer(taskUpload, args[1:])
	case "download":
		return parseTransfer(taskDownload, args[1:])
	case "version":
		return task{kind: taskVersion}, nil
	default:
		return task{}, newCLIError("cli.err.unknown_subcommand", args[0])
	}
}

func parseList(args []string) (task, error) {
	parsed := task{kind: taskList}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--group" || arg == "-g":
			value, next, err := nextValue(args, index, arg)
			if err != nil {
				return task{}, err
			}
			parsed.group = value
			index = next
		case strings.HasPrefix(arg, "--group="):
			parsed.group = strings.TrimPrefix(arg, "--group=")
		case arg == "--filter":
			value, next, err := nextValue(args, index, arg)
			if err != nil {
				return task{}, err
			}
			parsed.filter = value
			index = next
		case strings.HasPrefix(arg, "--filter="):
			parsed.filter = strings.TrimPrefix(arg, "--filter=")
		default:
			return task{}, newCLIError("cli.err.list_unsupported_arg", arg)
		}
	}
	return parsed, nil
}

func parseRun(args []string) (task, error) {
	parsed := task{kind: taskRun}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--":
			parsed.command = strings.Join(args[index+1:], " ")
			index = len(args)
		case arg == "-n" || arg == "--name":
			value, next, err := nextValue(args, index, arg)
			if err != nil {
				return task{}, err
			}
			parsed.names = appendNames(parsed.names, value)
			index = next
		case strings.HasPrefix(arg, "--name="):
			parsed.names = appendNames(parsed.names, strings.TrimPrefix(arg, "--name="))
		case arg == "--file":
			value, next, err := nextValue(args, index, arg)
			if err != nil {
				return task{}, err
			}
			parsed.file = value
			index = next
		case strings.HasPrefix(arg, "--file="):
			parsed.file = strings.TrimPrefix(arg, "--file=")
		case arg == "--fail-fast" || arg == "-ff":
			parsed.failFast = true
		default:
			return task{}, newCLIError("cli.err.run_unsupported_arg", arg)
		}
	}
	if len(parsed.names) == 0 {
		return task{}, newCLIError("cli.err.name_required")
	}
	if parsed.command != "" && parsed.file != "" {
		return task{}, newCLIError("cli.err.command_file_conflict")
	}
	if strings.TrimSpace(parsed.command) == "" && parsed.file == "" {
		return task{}, newCLIError("cli.err.command_required")
	}
	return parsed, nil
}

func parseTransfer(kind taskKind, args []string) (task, error) {
	parsed := task{kind: kind}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-n" || arg == "--name":
			value, next, err := nextValue(args, index, arg)
			if err != nil {
				return task{}, err
			}
			parsed.names = appendNames(parsed.names, value)
			index = next
		case strings.HasPrefix(arg, "--name="):
			parsed.names = appendNames(parsed.names, strings.TrimPrefix(arg, "--name="))
		case arg == "--local" || arg == "-l":
			value, next, err := nextValue(args, index, arg)
			if err != nil {
				return task{}, err
			}
			parsed.local = value
			index = next
		case strings.HasPrefix(arg, "--local="):
			parsed.local = strings.TrimPrefix(arg, "--local=")
		case arg == "--remote" || arg == "-r":
			value, next, err := nextValue(args, index, arg)
			if err != nil {
				return task{}, err
			}
			parsed.remote = value
			index = next
		case strings.HasPrefix(arg, "--remote="):
			parsed.remote = strings.TrimPrefix(arg, "--remote=")
		case arg == "-f" || arg == "--force":
			parsed.force = true
		case arg == "--fail-fast" || arg == "-ff":
			parsed.failFast = true
		default:
			return task{}, newCLIError("cli.err.transfer_unsupported_arg", commandName(kind), arg)
		}
	}
	if len(parsed.names) == 0 {
		return task{}, newCLIError("cli.err.name_required")
	}
	if kind == taskDownload && len(parsed.names) != 1 {
		return task{}, newCLIError("cli.err.download_batch_not_allowed")
	}
	if strings.TrimSpace(parsed.local) == "" {
		return task{}, newCLIError("cli.err.local_required")
	}
	if strings.TrimSpace(parsed.remote) == "" {
		return task{}, newCLIError("cli.err.remote_required")
	}
	return parsed, nil
}

func nextValue(args []string, index int, name string) (string, int, error) {
	next := index + 1
	if next >= len(args) || strings.TrimSpace(args[next]) == "" {
		return "", index, newCLIError("cli.err.option_value_required", name)
	}
	return args[next], next, nil
}

func appendNames(names []string, value string) []string {
	for _, name := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

func commandName(kind taskKind) string {
	switch kind {
	case taskUpload:
		return "upload"
	case taskDownload:
		return "download"
	default:
		return "command"
	}
}

func (c *Command) resolveConnections(names []string) ([]domain.Connection, bool) {
	connections, err := c.services.Connections.ResolveNames(names)
	if err != nil {
		fmt.Fprintf(c.stderr, c.t("cli.error_prefix"), c.cliError(err))
		return nil, false
	}
	return connections, true
}

func joinRemote(remoteDir string, name string) string {
	if strings.TrimSpace(remoteDir) == "" {
		return name
	}
	if strings.HasSuffix(remoteDir, "/") {
		return cleanRemote(remoteDir + name)
	}
	return cleanRemote(remoteDir + "/" + name)
}

func cleanRemote(remotePath string) string {
	if remotePath == "" {
		return remotePath
	}
	if strings.HasPrefix(remotePath, "~/") {
		return "~/" + path.Clean(strings.TrimPrefix(remotePath, "~/"))
	}
	return path.Clean(remotePath)
}

func exitCode(failed bool) int {
	if failed {
		return exitFailure
	}
	return exitSuccess
}

type cliError struct {
	key  string
	args []any
}

func newCLIError(key string, args ...any) error {
	return cliError{key: key, args: args}
}

func (e cliError) Error() string {
	return e.key
}

func (c *Command) cliError(err error) string {
	if err == nil {
		return ""
	}
	if parsed, ok := err.(cliError); ok {
		return c.t(parsed.key, parsed.args...)
	}
	return c.translator.Error(err)
}

func (c *Command) t(key string, args ...any) string {
	return c.translator.T(key, args...)
}

func (c *Command) writeHelp(writer io.Writer) {
	fmt.Fprint(writer, c.t("cli.help"))
}

type prefixWriter struct {
	writer      io.Writer
	prefix      string
	atLineStart bool
}

func newPrefixWriter(writer io.Writer, prefix string) io.Writer {
	return &prefixWriter{writer: writer, prefix: prefix, atLineStart: true}
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		if w.atLineStart {
			if _, err := io.WriteString(w.writer, w.prefix); err != nil {
				return written, err
			}
			w.atLineStart = false
		}
		newline := -1
		for index, char := range p {
			if char == '\n' {
				newline = index
				break
			}
		}
		if newline == -1 {
			n, err := w.writer.Write(p)
			written += n
			return written, err
		}
		n, err := w.writer.Write(p[:newline+1])
		written += n
		if err != nil {
			return written, err
		}
		w.atLineStart = true
		p = p[newline+1:]
	}
	return written, nil
}
