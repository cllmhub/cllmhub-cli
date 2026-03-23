package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	l, err := NewLogger(path)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer l.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}

func TestLogger_Log(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	l, _ := NewLogger(path)

	ts := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	l.Log(Entry{
		Timestamp: ts,
		RequestID: "req-123",
		Model:     "mistral-7b",
		Stream:    true,
		LatencyMs: 250,
		Tokens:    42,
	})
	l.Close()

	f, _ := os.Open(path)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected at least one line")
	}

	var entry Entry
	if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if entry.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", entry.RequestID, "req-123")
	}
	if entry.Model != "mistral-7b" {
		t.Errorf("Model = %q, want %q", entry.Model, "mistral-7b")
	}
	if !entry.Stream {
		t.Error("expected Stream=true")
	}
	if entry.LatencyMs != 250 {
		t.Errorf("LatencyMs = %d, want 250", entry.LatencyMs)
	}
	if entry.Tokens != 42 {
		t.Errorf("Tokens = %d, want 42", entry.Tokens)
	}
}

func TestLogger_LogSetsTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	l, _ := NewLogger(path)
	before := time.Now()
	l.Log(Entry{RequestID: "auto-ts"})
	l.Close()

	f, _ := os.Open(path)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan()

	var entry Entry
	json.Unmarshal(scanner.Bytes(), &entry)

	if entry.Timestamp.Before(before) {
		t.Error("expected auto-set timestamp to be >= test start time")
	}
}

func TestLogger_LogWithError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	l, _ := NewLogger(path)
	l.Log(Entry{
		RequestID: "err-req",
		Error:     "connection refused",
	})
	l.Close()

	f, _ := os.Open(path)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan()

	var entry Entry
	json.Unmarshal(scanner.Bytes(), &entry)

	if entry.Error != "connection refused" {
		t.Errorf("Error = %q, want %q", entry.Error, "connection refused")
	}
}

func TestLogger_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	l, _ := NewLogger(path)
	for i := 0; i < 5; i++ {
		l.Log(Entry{RequestID: "req"})
	}
	l.Close()

	f, _ := os.Open(path)
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if count != 5 {
		t.Errorf("expected 5 lines, got %d", count)
	}
}

func TestLogger_NilReceiver(t *testing.T) {
	var l *Logger
	// Should not panic
	l.Log(Entry{RequestID: "test"})
	if err := l.Close(); err != nil {
		t.Errorf("Close on nil: %v", err)
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	l, _ := NewLogger(path)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Log(Entry{RequestID: "concurrent"})
		}()
	}
	wg.Wait()
	l.Close()

	f, _ := os.Open(path)
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if count != 20 {
		t.Errorf("expected 20 lines, got %d", count)
	}
}
