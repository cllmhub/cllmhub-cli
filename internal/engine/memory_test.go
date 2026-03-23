package engine

import (
	"testing"
)

func TestParseVMStatValue(t *testing.T) {
	cases := []struct {
		line string
		want uint64
	}{
		{"Pages free:                               62986.", 62986},
		{"Pages active:                            735539.", 735539},
		{"", 0},
		{"NoFields", 0},
	}
	for _, tc := range cases {
		got := parseVMStatValue(tc.line)
		if got != tc.want {
			t.Errorf("parseVMStatValue(%q) = %d, want %d", tc.line, got, tc.want)
		}
	}
}

func TestParseMeminfoKB(t *testing.T) {
	cases := []struct {
		line string
		want uint64
	}{
		{"MemTotal:       16384000 kB", 16384000},
		{"MemAvailable:    8192000 kB", 8192000},
		{"", 0},
		{"NoValue", 0},
	}
	for _, tc := range cases {
		got := parseMeminfoKB(tc.line)
		if got != tc.want {
			t.Errorf("parseMeminfoKB(%q) = %d, want %d", tc.line, got, tc.want)
		}
	}
}

func TestCheckMemory(t *testing.T) {
	mem, err := CheckMemory()
	if err != nil {
		t.Skipf("CheckMemory not supported on this platform: %v", err)
	}
	if mem.TotalBytes == 0 {
		t.Error("expected non-zero TotalBytes")
	}
	if mem.AvailableBytes == 0 {
		t.Error("expected non-zero AvailableBytes")
	}
	if mem.AvailableBytes > mem.TotalBytes {
		t.Errorf("AvailableBytes (%d) > TotalBytes (%d)", mem.AvailableBytes, mem.TotalBytes)
	}
}
