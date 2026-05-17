package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPriorPubDates(t *testing.T) {
	dir := t.TempDir()
	feed := filepath.Join(dir, "feed.xml")

	content := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <item>
      <title>Three</title>
      <link>https://boringstack.org/issues/2026-05-12-sqlite-is-a-database.html</link>
      <guid isPermaLink="true">https://boringstack.org/issues/2026-05-12-sqlite-is-a-database.html</guid>
      <pubDate>Tue, 12 May 2026 21:15:00 +0000</pubDate>
    </item>
    <item>
      <title>Two</title>
      <link>https://boringstack.org/issues/2026-05-05-one-vps-one-binary-one-database.html</link>
      <guid isPermaLink="true">https://boringstack.org/issues/2026-05-05-one-vps-one-binary-one-database.html</guid>
      <pubDate>Tue, 05 May 2026 00:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`
	if err := os.WriteFile(feed, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got := loadPriorPubDates(feed)
	want := map[string]string{
		"2026-05-12-sqlite-is-a-database":              "Tue, 12 May 2026 21:15:00 +0000",
		"2026-05-05-one-vps-one-binary-one-database":   "Tue, 05 May 2026 00:00:00 +0000",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestLoadPriorPubDates_MissingFileNoOp(t *testing.T) {
	got := loadPriorPubDates(filepath.Join(t.TempDir(), "nope.xml"))
	if got != nil && len(got) != 0 {
		t.Errorf("expected nil/empty map for missing file, got %v", got)
	}
}
