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
)
