package xmltv

import (
	"encoding/xml"
	"testing"
	"time"
)

func TestXMLTVTimeParse(t *testing.T) {
	tests := []struct {
		input string
		year  int
		month time.Month
		day   int
		hour  int
		min   int
	}{
		{"20260326200000 +0200", 2026, time.March, 26, 18, 0}, // UTC
		{"20260326200000 +0300", 2026, time.March, 26, 17, 0}, // UTC
		{"20260101120000 +0200", 2026, time.January, 1, 10, 0},
	}

	for _, tt := range tests {
		var xt XMLTVTime
		attr := xml.Attr{Value: tt.input}
		if err := xt.UnmarshalXMLAttr(attr); err != nil {
			t.Errorf("parse %q: %v", tt.input, err)
			continue
		}

		if xt.Year() != tt.year || xt.Month() != tt.month || xt.Day() != tt.day {
			t.Errorf("parse %q: date = %v, want %d-%d-%d", tt.input, xt.Time, tt.year, tt.month, tt.day)
		}
		if xt.Hour() != tt.hour || xt.Minute() != tt.min {
			t.Errorf("parse %q: time = %02d:%02d, want %02d:%02d", tt.input, xt.Hour(), xt.Minute(), tt.hour, tt.min)
		}
	}
}

func TestXMLTVTimeParseNoTZ(t *testing.T) {
	var xt XMLTVTime
	attr := xml.Attr{Value: "20260326200000"}
	if err := xt.UnmarshalXMLAttr(attr); err != nil {
		t.Fatalf("parse without TZ: %v", err)
	}
	if xt.Hour() != 20 {
		t.Errorf("hour = %d, want 20", xt.Hour())
	}
}

func TestXMLTVTimeParseInvalid(t *testing.T) {
	var xt XMLTVTime
	attr := xml.Attr{Value: "invalid"}
	if err := xt.UnmarshalXMLAttr(attr); err == nil {
		t.Error("expected error for invalid input")
	}
	_ = xt
}
