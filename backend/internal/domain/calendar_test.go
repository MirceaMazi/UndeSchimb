package domain

import (
	"testing"
	"time"
)

func TestBucharestLocationIncludesSummerTimeInMinimalImages(t *testing.T) {
	instant := time.Date(2026, 8, 20, 13, 0, 0, 0, time.UTC)
	local := instant.In(BucharestLocation())
	_, offset := local.Zone()
	if local.Hour() != 16 || offset != 3*60*60 {
		t.Fatalf("expected 16:00 EEST, got %s with offset %d", local, offset)
	}
}
