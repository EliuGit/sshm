//go:build windows

package ssh

func watchWindowChanges(_ uintptr, _ func(cols, rows int) error) func() {
	return func() {}
}
