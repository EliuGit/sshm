package domain

import (
	"errors"
	"fmt"
)

type TranslatableError interface {
	error
	TranslationKey() string
	TranslationArgs() []any
}

type translatableError struct {
	key  string
	args []any
}

func (e translatableError) Error() string {
	if len(e.args) == 0 {
		return e.key
	}
	return fmt.Sprintf("%s %v", e.key, e.args)
}

func (e translatableError) TranslationKey() string {
	return e.key
}

func (e translatableError) TranslationArgs() []any {
	return append([]any(nil), e.args...)
}

func (e translatableError) Is(target error) bool {
	other, ok := target.(translatableError)
	if !ok {
		return false
	}
	if e.key != other.key || len(e.args) != len(other.args) {
		return false
	}
	for index := range e.args {
		if fmt.Sprint(e.args[index]) != fmt.Sprint(other.args[index]) {
			return false
		}
	}
	return true
}

func NewTranslatableError(key string, args ...any) error {
	return translatableError{key: key, args: append([]any(nil), args...)}
}

func NewConnectionNameNotFoundError(name string) error {
	return NewTranslatableError("err.connection_name_not_found", name)
}

func NewConnectionNameDuplicatedError(name string) error {
	return NewTranslatableError("err.connection_name_duplicated", name)
}

type FileOperation string

const (
	FileOpList     FileOperation = "list"
	FileOpStat     FileOperation = "stat"
	FileOpMkdir    FileOperation = "mkdir"
	FileOpRemove   FileOperation = "remove"
	FileOpRename   FileOperation = "rename"
	FileOpUpload   FileOperation = "upload"
	FileOpDownload FileOperation = "download"
	FileOpResolve  FileOperation = "resolve"
)

// FileError 统一承载本地/远端文件操作失败上下文。
//
// 设计说明：
// 1. app / ui / transport 三层都只传递这一种文件错误包装，避免继续依赖字符串前缀推断场景。
// 2. Path / TargetPath 直接记录当前操作关注的路径范围，便于 UI / CLI 在翻译时补齐“在哪失败”的上下文。
// 3. Cause 仍然通过 Unwrap 暴露底层错误，后续若要按 SSH/SFTP/OS 错误类型细分处理，可以在不改上层签名的前提下扩展。
type FileError struct {
	Panel      FilePanel
	Op         FileOperation
	Path       string
	TargetPath string
	Cause      error
}

func (e *FileError) Error() string {
	if e == nil {
		return ""
	}
	if e.TargetPath != "" {
		return fmt.Sprintf("%s %s %s -> %s: %v", e.Panel.String(), e.Op, e.Path, e.TargetPath, e.Cause)
	}
	return fmt.Sprintf("%s %s %s: %v", e.Panel.String(), e.Op, e.Path, e.Cause)
}

func (e *FileError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func WrapFileError(panel FilePanel, op FileOperation, path string, targetPath string, err error) error {
	if err == nil {
		return nil
	}
	var fileErr *FileError
	if errors.As(err, &fileErr) {
		return err
	}
	return &FileError{
		Panel:      panel,
		Op:         op,
		Path:       path,
		TargetPath: targetPath,
		Cause:      err,
	}
}

var (
	ErrConnectionNotFound       = errors.New("connection not found")
	ErrConnectionSecretNotFound = errors.New("connection secret not found")

	ErrConnectionNameRequired = NewTranslatableError("err.connection_name_required")
	ErrNameRequired           = NewTranslatableError("err.name_required")
	ErrHostRequired           = NewTranslatableError("err.host_required")
	ErrUsernameRequired       = NewTranslatableError("err.username_required")
	ErrPortRange              = NewTranslatableError("err.port_range")
	ErrPasswordRequired       = NewTranslatableError("err.password_required")
	ErrUnsupportedAuthType    = NewTranslatableError("err.unsupported_auth_type")
	ErrInteractiveTerminal    = NewTranslatableError("err.interactive_terminal")
	ErrUnsupportedLanguage    = NewTranslatableError("err.unsupported_language")
	ErrDatabasePathRequired   = NewTranslatableError("err.database_path_required")
	ErrGroupNameRequired      = NewTranslatableError("err.group_name_required")
	ErrGroupRequired          = NewTranslatableError("err.group_required")
	ErrFileNameRequired       = NewTranslatableError("err.file_name_required")
	ErrPathRequired           = NewTranslatableError("err.path_required")
	ErrFileNamePathSeparator  = NewTranslatableError("err.file_name_path_separator")
	ErrRemoteDeleteProtected  = NewTranslatableError("err.remote_delete_root_protected")
	ErrBrowserSessionNotReady = NewTranslatableError("err.browser_session_not_ready")
)
