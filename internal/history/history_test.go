package history

import (
	"os"
	"testing"
	"time"
)

// Since I am in a linux environment in the sandbox:
func TestAddAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "huh_test_config")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override the function locally if I could, but I can't.
	// Let's set XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", originalXDG)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Verify path
	path, err := GetHistoryPath()
	if err != nil {
		t.Fatal(err)
	}
	// On linux, it should be tmpDir/huh/history.json
	// On mac, it might ignore XDG.
	// Let's see what happens.
    t.Logf("History path: %s", path)

	item1 := Item{Question: "Q1", Answer: "A1", Timestamp: time.Now()}
	if err := Add(item1); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	items, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}
	if items[0].Question != "Q1" {
		t.Errorf("Expected Q1, got %s", items[0].Question)
	}

	// Test Limit
	for i := 0; i < 15; i++ {
		// We need distinct items to avoid deduplication logic
		// Or we can rely on timestamp? No, dedupe logic checks Q and A.
		// So we need distinct Q/A or it will skip adding.
		// Wait, my dedupe logic checks only the *last* item.
		// So if I add identical items in sequence, they are deduped.
		// But if I alternate, they are kept.
		// Let's use unique questions.
		Add(Item{Question: string(rune(i)), Answer: "A", Timestamp: time.Now()})
	}

	items, _ = Load()
	if len(items) != 10 {
		t.Errorf("Expected 10 items, got %d", len(items))
	}
}
