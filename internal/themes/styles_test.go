package themes

import "testing"

func TestResolveStylesSupportsBuiltInThemes(t *testing.T) {
	t.Parallel()

	dark, err := ResolveStyles(DefaultName)
	if err != nil {
		t.Fatalf("ResolveStyles(%q) error = %v", DefaultName, err)
	}
	light, err := ResolveStyles(LightName)
	if err != nil {
		t.Fatalf("ResolveStyles(%q) error = %v", LightName, err)
	}

	if dark.Text.GetForeground() == light.Text.GetForeground() {
		t.Fatal("dark/light Text 前景色不应相同")
	}
	if dark.Selection.GetBackground() == light.Selection.GetBackground() {
		t.Fatal("dark/light Selection 背景色不应相同")
	}
}

func TestResolveStylesSupportsAdditionalDarkThemes(t *testing.T) {
	t.Parallel()

	dracula, err := ResolveStyles(DraculaName)
	if err != nil {
		t.Fatalf("ResolveStyles(%q) error = %v", DraculaName, err)
	}
	nord, err := ResolveStyles(NordName)
	if err != nil {
		t.Fatalf("ResolveStyles(%q) error = %v", NordName, err)
	}

	if dracula.Selection.GetBackground() == nord.Selection.GetBackground() {
		t.Fatal("dracula/nord Selection 背景色不应相同")
	}
	if dracula.PageTitle.GetForeground() == nord.PageTitle.GetForeground() {
		t.Fatal("dracula/nord 标题前景色不应相同")
	}
}

func TestSupportedNamesIncludesAllBuiltInThemes(t *testing.T) {
	t.Parallel()

	got := SupportedNames()
	want := []string{DefaultName, DraculaName, LightName, NordName}
	if len(got) != len(want) {
		t.Fatalf("SupportedNames() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("SupportedNames()[%d] = %q, want %q (all=%v)", index, got[index], want[index], got)
		}
	}
}

func TestResolveStylesRejectsUnknownTheme(t *testing.T) {
	t.Parallel()

	if _, err := ResolveStyles("unknown"); err == nil {
		t.Fatal("ResolveStyles(unknown) error = nil, want non-nil")
	}
}
