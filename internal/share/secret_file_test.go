package share

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteSecretFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	for _, want := range []string{"first", "replaced"} {
		if err := WriteSecretFile(path, []byte(want)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("文件内容 = %q，期望 %q", got, want)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Fatalf("文件权限 = %o，期望 600", mode)
	}
}
