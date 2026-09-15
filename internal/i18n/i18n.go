// Package i18n 提供应用启动时的语言检测和运行时文案翻译。
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/jeandeaual/go-locale"
)

const languageEnvVarKey = "SSHM_LANG"

type language uint8

const (
	english language = iota
	chinese
)

var (
	currentLanguage = english
	initializeOnce  sync.Once
)

// Init 在程序启动时检测并固定本次运行使用的语言。
func Init() {
	initializeOnce.Do(func() {
		currentLanguage = resolveLanguage(os.Getenv(languageEnvVarKey), locale.GetLocale)
	})
}

// T 返回当前语言的文案，并在提供参数时完成格式化。
func T(englishText string, args ...any) string {
	Init()
	return translate(currentLanguage, englishText, args...)
}

func resolveLanguage(envValue string, systemLocale func() (string, error)) language {
	value := envValue
	if value == "" {
		var err error
		value, err = systemLocale()
		if err != nil || value == "" {
			return english
		}
	}
	if strings.HasPrefix(strings.ToLower(value), "zh") {
		return chinese
	}
	return english
}

func translate(lang language, englishText string, args ...any) string {
	text := englishText
	if lang == chinese {
		if translated, ok := chineseTranslations[englishText]; ok {
			text = translated
		}
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}
