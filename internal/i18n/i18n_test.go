package i18n

import (
	"errors"
	"testing"
)

func TestResolveLanguage(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		system     string
		systemErr  error
		want       language
		wantCalled bool
	}{
		{name: "环境变量中文简写", env: "zh", system: "en-US", want: chinese},
		{name: "环境变量中文", env: "zh-CN", system: "en-US", want: chinese},
		{name: "环境变量忽略大小写", env: "ZH_TW", system: "en-US", want: chinese},
		{name: "环境变量英文", env: "en-US", system: "zh-CN", want: english},
		{name: "其他环境变量", env: "ja-JP", system: "zh-CN", want: english},
		{name: "系统中文", system: "zh-CN", want: chinese, wantCalled: true},
		{name: "系统英文", system: "en-US", want: english, wantCalled: true},
		{name: "系统为空", want: english, wantCalled: true},
		{name: "系统获取失败", systemErr: errors.New("failed"), want: english, wantCalled: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			got := resolveLanguage(test.env, func() (string, error) {
				called = true
				return test.system, test.systemErr
			})
			if got != test.want || called != test.wantCalled {
				t.Fatalf("语言 = %v, 系统语言调用 = %v; want %v, %v", got, called, test.want, test.wantCalled)
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	if got := translate(chinese, "Read %d items", 3); got != "已读取 3 项" {
		t.Fatalf("中文翻译 = %q", got)
	}
	if got := translate(english, "Read %d items", 3); got != "Read 3 items" {
		t.Fatalf("英文翻译 = %q", got)
	}
	if got := translate(chinese, "Unknown text"); got != "Unknown text" {
		t.Fatalf("缺失翻译未回退英文: %q", got)
	}
}
