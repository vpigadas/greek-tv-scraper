package registry

import "testing"

func TestChannelByID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"ert1", "ERT1"},
		{"ant1", "ANT1"},
		{"mega", "MEGA"},
		{"alpha", "Alpha"},
		{"cosmote_sport_1", "Cosmote Sport 1"},
		{"rik1", "RIK 1"},
		{"CNN", "CNN"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		ch := ChannelByID(tt.id)
		if tt.want == "" {
			if ch != nil {
				t.Errorf("ChannelByID(%q) = %v, want nil", tt.id, ch)
			}
			continue
		}
		if ch == nil {
			t.Errorf("ChannelByID(%q) = nil, want %q", tt.id, tt.want)
			continue
		}
		if ch.Name != tt.want {
			t.Errorf("ChannelByID(%q).Name = %q, want %q", tt.id, ch.Name, tt.want)
		}
	}
}

func TestChannelCount(t *testing.T) {
	if len(Channels) < 200 {
		t.Errorf("len(Channels) = %d, want >= 200", len(Channels))
	}
}

func TestUniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, ch := range Channels {
		if seen[ch.ID] {
			t.Errorf("duplicate channel ID: %q", ch.ID)
		}
		seen[ch.ID] = true
	}
}

func TestGroupsNotEmpty(t *testing.T) {
	groups := make(map[string]int)
	for _, ch := range Channels {
		groups[ch.Group]++
	}

	expected := []string{"ert", "national", "regional", "cypriot", "news", "commercial-sport", "commercial-cinema", "commercial-kids", "commercial-other"}
	for _, g := range expected {
		if groups[g] == 0 {
			t.Errorf("group %q has no channels", g)
		}
	}
}
