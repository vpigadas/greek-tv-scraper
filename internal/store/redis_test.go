package store

import (
	"testing"
	"time"
)

func TestBroadcastDate(t *testing.T) {
	athens, _ := time.LoadLocation("Europe/Athens")

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{
			name: "afternoon show belongs to same day",
			time: time.Date(2026, 3, 26, 12, 0, 0, 0, athens), // 12:00 Athens
			want: "2026-03-26",
		},
		{
			name: "late night show at 01:00 belongs to previous broadcast day",
			time: time.Date(2026, 3, 27, 1, 0, 0, 0, athens), // 01:00 Athens on 27th
			want: "2026-03-26",                                 // belongs to 26th broadcast day
		},
		{
			name: "show at 05:59 belongs to previous broadcast day",
			time: time.Date(2026, 3, 27, 5, 59, 0, 0, athens),
			want: "2026-03-26",
		},
		{
			name: "show at 06:00 belongs to current broadcast day",
			time: time.Date(2026, 3, 27, 6, 0, 0, 0, athens),
			want: "2026-03-27",
		},
		{
			name: "show at 06:01 belongs to current broadcast day",
			time: time.Date(2026, 3, 27, 6, 1, 0, 0, athens),
			want: "2026-03-27",
		},
		{
			name: "UTC time converts correctly — 22:00 UTC in summer = 01:00+1 Athens",
			time: time.Date(2026, 7, 15, 22, 0, 0, 0, time.UTC), // 01:00 Athens on Jul 16
			want: "2026-07-15",                                    // belongs to Jul 15 broadcast day
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BroadcastDate(tt.time, athens)
			if got != tt.want {
				t.Errorf("BroadcastDate(%v) = %q, want %q", tt.time, got, tt.want)
			}
		})
	}
}
