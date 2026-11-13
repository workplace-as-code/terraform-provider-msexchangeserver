package resources

import "testing"

func TestParseActiveSyncRuleName(t *testing.T) {
	tests := []struct {
		name, query, characteristic string
		ok                          bool
	}{
		{"iPhone (DeviceModel)", "iPhone", "DeviceModel", true},
		{"iOS 17.5 21F79 (DeviceOS)", "iOS 17.5 21F79", "DeviceOS", true},
		// Only the last group is the characteristic.
		{"Android (Pixel 8) (devicetype)", "Android (Pixel 8)", "DeviceType", true},
		{"iPhone", "", "", false},
		{"iPhone (Model)", "", "", false},
		{" (DeviceModel)", "", "", false},
	}
	for _, tt := range tests {
		query, characteristic, ok := parseActiveSyncRuleName(tt.name)
		if query != tt.query || characteristic != tt.characteristic || ok != tt.ok {
			t.Errorf("parseActiveSyncRuleName(%q) = %q, %q, %v; want %q, %q, %v",
				tt.name, query, characteristic, ok, tt.query, tt.characteristic, tt.ok)
		}
	}
}
