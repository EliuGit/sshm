package ssh

import "testing"

func TestIsRemoteDeleteProtectedPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path      string
		protected bool
	}{
		{path: "/", protected: true},
		{path: "/var", protected: true},
		{path: "/etc/hosts", protected: false},
		{path: "/srv/app/logs", protected: false},
		{path: "/tmp.txt", protected: true},
	}

	for _, tc := range cases {
		if got := isRemoteDeleteProtectedPath(tc.path); got != tc.protected {
			t.Fatalf("isRemoteDeleteProtectedPath(%q) = %v, want %v", tc.path, got, tc.protected)
		}
	}
}
