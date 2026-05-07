package ssh

import (
	"errors"
	"testing"
)

func TestDetectTerminalSizeChange(t *testing.T) {
	t.Parallel()

	last := terminalSize{cols: 120, rows: 30}

	cases := []struct {
		name    string
		last    terminalSize
		cols    int
		rows    int
		err     error
		want    terminalSize
		changed bool
	}{
		{
			name:    "resize changed",
			last:    last,
			cols:    140,
			rows:    36,
			want:    terminalSize{cols: 140, rows: 36},
			changed: true,
		},
		{
			name:    "size unchanged",
			last:    last,
			cols:    120,
			rows:    30,
			want:    last,
			changed: false,
		},
		{
			name:    "read size failed",
			last:    last,
			cols:    140,
			rows:    36,
			err:     errors.New("boom"),
			want:    last,
			changed: false,
		},
		{
			name:    "invalid cols",
			last:    last,
			cols:    0,
			rows:    36,
			want:    last,
			changed: false,
		},
		{
			name:    "invalid rows",
			last:    last,
			cols:    140,
			rows:    -1,
			want:    last,
			changed: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, changed := detectTerminalSizeChange(tc.last, tc.cols, tc.rows, tc.err)
			if got != tc.want {
				t.Fatalf("detectTerminalSizeChange() size = %+v, want %+v", got, tc.want)
			}
			if changed != tc.changed {
				t.Fatalf("detectTerminalSizeChange() changed = %v, want %v", changed, tc.changed)
			}
		})
	}
}
