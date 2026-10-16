package credits

import "testing"

func TestAttemptKeysScopeIdempotency(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"reserve attempt 1", AttemptReserveKey("t1", 1), "task:t1:1:reserve"},
		{"settle attempt 2", AttemptSettleKey("t1", 2), "task:t1:2:settle"},
		{"release attempt 3", AttemptReleaseKey("t1", 3), "task:t1:3:release"},
		{"legacy reserve", AttemptReserveKey("t1", 0), ReserveKey("t1")},
		{"legacy settle", AttemptSettleKey("t1", 0), SettleKey("t1")},
		{"legacy release", AttemptReleaseKey("t1", 0), ReleaseKey("t1")},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
