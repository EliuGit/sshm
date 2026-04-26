//go:build !windows

package ssh

func resetInteractiveInput() error {
	return nil
}
