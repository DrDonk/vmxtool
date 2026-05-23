// SPDX-FileCopyrightText: © 2025-2026 David Parsons
// SPDX-License-Identifier: MIT
//
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test file content based on real macOS VMX
const testVMXContent = `# OC4VM VERSION
# 3.0.1
# 212ced9
# SPDX-FileCopyrightText: © 2023-2026 David Parsons
# SPDX-License-Identifier: MIT
.encoding = "UTF-8"

__OC4VM_GuestInfo__ = ""
guestinfo.oc4vm.version = "3.0.1"
guestinfo.oc4vm.revision = "212ced9"

__macOS_Settings__ = ""
guestOS = "darwin24-64"                   # Set macOS guest version
smc.present = "FALSE"                     # SMC is emulated in OpenCore
system-id.enable = "TRUE"                 # Pass system UUID into guest

__CPU_Settings__ = ""
cpuid.coresPerSocket = "1"
numvcpus = "4"
featMask.vm.cpuid.AMD = "Min:1"           # Check CPU as OC4VM has Intel and AMD variants

__Serial_Port_Settings__ = ""
serial0.present = "FALSE"
serial0.fileName = "serial.log"

__VMware_Build__ = ""
vmx.buildType = "release"                # VMware build release/debug/stats
`

// createTestFile creates a temporary test VMX file
func createTestFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test-*.vmx")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	return tmpFile.Name()
}

// TestLoadDictionary tests loading a VMX file
func TestLoadDictionary(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, err := LoadDictionary(filename)
	if err != nil {
		t.Fatalf("Failed to load dictionary: %v", err)
	}

	if dict.Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, dict.Filename)
	}

	if len(dict.Entries) == 0 {
		t.Error("Expected entries to be loaded")
	}

	// Check that we have the expected number of key-value pairs
	kvCount := 0
	for _, entry := range dict.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			kvCount++
		}
	}

	if kvCount < 10 {
		t.Errorf("Expected at least 10 key-value pairs, got %d", kvCount)
	}
}

// TestCaseInsensitiveQuery tests case-insensitive key lookup
func TestCaseInsensitiveQuery(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	tests := []struct {
		key      string
		expected string
	}{
		{"vmx.buildType", "release"},
		{"VMX.BUILDTYPE", "release"},
		{"vmx.buildtype", "release"},
		{"VmX.BuIlDtYpE", "release"},
		{"guestOS", "darwin24-64"},
		{"GUESTOS", "darwin24-64"},
		{"numvcpus", "4"},
		{"NUMVCPUS", "4"},
	}

	for _, test := range tests {
		value, err := dict.Query(test.key)
		if err != nil {
			t.Errorf("Query(%s) failed: %v", test.key, err)
		}
		if value != test.expected {
			t.Errorf("Query(%s) = %s, expected %s", test.key, value, test.expected)
		}
	}
}

// TestInlineComments tests that inline comments are preserved
func TestInlineComments(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Find an entry with an inline comment
	var commentEntry *Entry
	for _, entry := range dict.Entries {
		if entry.Key == "guestOS" {
			commentEntry = entry
			break
		}
	}

	if commentEntry == nil {
		t.Fatal("Could not find guestOS entry")
	}

	if commentEntry.InlineComment == "" {
		t.Error("Expected inline comment to be preserved")
	}

	if !strings.Contains(commentEntry.InlineComment, "# Set macOS guest version") {
		t.Errorf("Expected inline comment to contain '# Set macOS guest version', got: %s", commentEntry.InlineComment)
	}
}

// TestAddAndQuery tests adding a new key-value pair
func TestAddAndQuery(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Add a new key
	err := dict.Add("test.newkey", "testvalue")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Query it back
	value, err := dict.Query("test.newkey")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if value != "testvalue" {
		t.Errorf("Expected 'testvalue', got '%s'", value)
	}

	// Try to add duplicate (should fail)
	err = dict.Add("test.newkey", "another")
	if err == nil {
		t.Error("Expected error when adding duplicate key")
	}
}

// TestSetAndUpdate tests setting and updating values
func TestSetAndUpdate(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Update existing key
	dict.Set("numvcpus", "8")
	value, _ := dict.Query("numvcpus")
	if value != "8" {
		t.Errorf("Expected '8', got '%s'", value)
	}

	// Set new key (case-insensitive check)
	dict.Set("NUMVCPUS", "16")
	value, _ = dict.Query("numvcpus")
	if value != "16" {
		t.Errorf("Expected '16' (should update existing), got '%s'", value)
	}

	// Add completely new key
	dict.Set("test.brand.new", "value")
	value, _ = dict.Query("test.brand.new")
	if value != "value" {
		t.Errorf("Expected 'value', got '%s'", value)
	}
}

// TestRemove tests removing keys
func TestRemove(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Remove existing key (case-insensitive)
	err := dict.Remove("NUMVCPUS")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify it's gone
	_, err = dict.Query("numvcpus")
	if err == nil {
		t.Error("Expected error when querying removed key")
	}

	// Try to remove non-existent key
	err = dict.Remove("nonexistent.key")
	if err == nil {
		t.Error("Expected error when removing non-existent key")
	}
}

// TestPrefixOperations tests add/remove prefix
func TestPrefixOperations(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Add prefix
	err := dict.AddPrefix("numvcpus", "test.")
	if err != nil {
		t.Fatalf("AddPrefix failed: %v", err)
	}

	// Verify new key exists
	value, err := dict.Query("test.numvcpus")
	if err != nil {
		t.Fatalf("Query after AddPrefix failed: %v", err)
	}
	if value != "4" {
		t.Errorf("Expected '4', got '%s'", value)
	}

	// Remove prefix (case-insensitive)
	err = dict.RemovePrefix("TEST.numvcpus", "test.")
	if err != nil {
		t.Fatalf("RemovePrefix failed: %v", err)
	}

	// Verify original key is back
	value, err = dict.Query("numvcpus")
	if err != nil {
		t.Fatalf("Query after RemovePrefix failed: %v", err)
	}
	if value != "4" {
		t.Errorf("Expected '4', got '%s'", value)
	}
}

// TestListKeys tests listing keys with prefix filter
func TestListKeys(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// List all keys
	allKeys := dict.ListKeys("")
	if len(allKeys) < 10 {
		t.Errorf("Expected at least 10 keys, got %d", len(allKeys))
	}

	// List keys with prefix (case-insensitive)
	serialKeys := dict.ListKeys("SERIAL0.")
	if len(serialKeys) < 2 {
		t.Errorf("Expected at least 2 serial0 keys, got %d", len(serialKeys))
	}

	// Verify all returned keys have the prefix
	for _, key := range serialKeys {
		if !strings.HasPrefix(strings.ToLower(key), "serial0.") {
			t.Errorf("Key '%s' doesn't have prefix 'serial0.'", key)
		}
	}
}

// TestCommentAndUncomment tests commenting out and uncommenting entries
func TestCommentAndUncomment(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Comment out a key
	err := dict.CommentOut("numvcpus")
	if err != nil {
		t.Fatalf("CommentOut failed: %v", err)
	}

	// Verify it's commented
	_, err = dict.Query("numvcpus")
	if err == nil {
		t.Error("Expected error when querying commented key")
	}

	// Uncomment it (case-insensitive)
	err = dict.Uncomment("NUMVCPUS")
	if err != nil {
		t.Fatalf("Uncomment failed: %v", err)
	}

	// Verify it's back
	value, err := dict.Query("numvcpus")
	if err != nil {
		t.Fatalf("Query after Uncomment failed: %v", err)
	}
	if value != "4" {
		t.Errorf("Expected '4', got '%s'", value)
	}
}

// TestInsertComment tests inserting comments
func TestInsertComment(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	initialCount := len(dict.Entries)

	// Insert comment before
	err := dict.InsertCommentBefore("numvcpus", "This is a test comment")
	if err != nil {
		t.Fatalf("InsertCommentBefore failed: %v", err)
	}

	if len(dict.Entries) != initialCount+1 {
		t.Errorf("Expected %d entries, got %d", initialCount+1, len(dict.Entries))
	}

	// Find the comment
	idx := dict.findEntryIndex("numvcpus")
	if idx <= 0 {
		t.Fatal("Could not find numvcpus entry")
	}

	commentEntry := dict.Entries[idx-1]
	if !commentEntry.IsComment {
		t.Error("Expected entry before numvcpus to be a comment")
	}

	if !strings.Contains(commentEntry.Original, "This is a test comment") {
		t.Errorf("Comment doesn't contain expected text: %s", commentEntry.Original)
	}
}

// TestInsertBlankLines tests inserting blank lines
func TestInsertBlankLines(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	initialCount := len(dict.Entries)

	// Insert 2 blank lines before
	err := dict.InsertBlankLinesBefore("numvcpus", 2)
	if err != nil {
		t.Fatalf("InsertBlankLinesBefore failed: %v", err)
	}

	if len(dict.Entries) != initialCount+2 {
		t.Errorf("Expected %d entries, got %d", initialCount+2, len(dict.Entries))
	}

	// Verify they're blank
	idx := dict.findEntryIndex("numvcpus")
	if idx < 2 {
		t.Fatal("Not enough entries before numvcpus")
	}

	for i := idx - 2; i < idx; i++ {
		if !dict.Entries[i].IsBlank {
			t.Errorf("Entry at index %d should be blank", i)
		}
	}
}

// TestRemoveComment tests removing comments
func TestRemoveComment(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// First insert a comment
	dict.InsertCommentBefore("numvcpus", "Test comment to remove")

	initialCount := len(dict.Entries)

	// Remove it
	err := dict.RemoveCommentBefore("numvcpus")
	if err != nil {
		t.Fatalf("RemoveCommentBefore failed: %v", err)
	}

	if len(dict.Entries) != initialCount-1 {
		t.Errorf("Expected %d entries, got %d", initialCount-1, len(dict.Entries))
	}
}

// TestRemoveBlankLines tests removing blank lines
func TestRemoveBlankLines(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// First insert blank lines
	dict.InsertBlankLinesBefore("numvcpus", 3)

	initialCount := len(dict.Entries)

	// Remove 2 of them
	err := dict.RemoveBlankBefore("numvcpus", 2)
	if err != nil {
		t.Fatalf("RemoveBlankBefore failed: %v", err)
	}

	if len(dict.Entries) != initialCount-2 {
		t.Errorf("Expected %d entries, got %d", initialCount-2, len(dict.Entries))
	}
}

// TestSearch tests searching for keys and values
func TestSearch(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Search keys
	results, err := dict.Search("serial", false)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for 'serial', got %d", len(results))
	}

	// Search values
	results, err = dict.Search("FALSE", true)
	if err != nil {
		t.Fatalf("Search values failed: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for 'FALSE', got %d", len(results))
	}
}

// TestSaveAndReload tests saving and reloading
func TestSaveAndReload(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Make some changes
	dict.Set("test.key", "test value")
	dict.Set("numvcpus", "16")

	// Save
	err := dict.Save(filename)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload
	dict2, err := LoadDictionary(filename)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Verify changes persisted
	value, _ := dict2.Query("test.key")
	if value != "test value" {
		t.Errorf("Expected 'test value', got '%s'", value)
	}

	value, _ = dict2.Query("numvcpus")
	if value != "16" {
		t.Errorf("Expected '16', got '%s'", value)
	}
}

// TestQuoteEscaping tests that quotes in values are properly escaped
func TestQuoteEscaping(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Add a value with quotes
	dict.Set("test.quotes", `He said "hello"`)

	// Save and reload
	dict.Save(filename)
	dict2, _ := LoadDictionary(filename)

	// Verify quotes are preserved
	value, _ := dict2.Query("test.quotes")
	if value != `He said "hello"` {
		t.Errorf("Expected 'He said \"hello\"', got '%s'", value)
	}
}

// TestUTF8Support tests UTF-8 character support
func TestUTF8Support(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	tests := []struct {
		key   string
		value string
	}{
		{"test.emoji", "Hello 👋 World"},
		{"test.chinese", "你好世界"},
		{"test.french", "Café"},
		{"test.german", "Müller"},
	}

	for _, test := range tests {
		dict.Set(test.key, test.value)
	}

	// Save and reload
	dict.Save(filename)
	dict2, _ := LoadDictionary(filename)

	// Verify UTF-8 values are preserved
	for _, test := range tests {
		value, err := dict2.Query(test.key)
		if err != nil {
			t.Errorf("Query(%s) failed: %v", test.key, err)
		}
		if value != test.value {
			t.Errorf("Expected '%s', got '%s'", test.value, value)
		}
	}
}

// TestValidate tests the validation function
func TestValidate(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Original file should have no issues
	issues := dict.Validate()
	if len(issues) > 0 {
		t.Logf("Validation issues found (expected for this test file): %v", issues)
	}

	// Add a duplicate key (case-insensitive)
	dict.Entries = append(dict.Entries, &Entry{
		Key:      "NUMVCPUS",
		Value:    "8",
		Original: "NUMVCPUS = \"8\"",
	})

	issues = dict.Validate()
	foundDuplicate := false
	for _, issue := range issues {
		if strings.Contains(strings.ToLower(issue), "duplicate") {
			foundDuplicate = true
			break
		}
	}

	if !foundDuplicate {
		t.Error("Expected to find duplicate key issue")
	}
}

// TestExportImportJSON tests JSON export and import
func TestExportImportJSON(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Export to JSON
	jsonStr, err := dict.ExportToJSON()
	if err != nil {
		t.Fatalf("ExportToJSON failed: %v", err)
	}

	if !strings.Contains(jsonStr, "numvcpus") {
		t.Error("Expected JSON to contain 'numvcpus'")
	}

	// Create new dict and import
	dict2 := &Dictionary{}
	err = dict2.ImportFromJSON(jsonStr)
	if err != nil {
		t.Fatalf("ImportFromJSON failed: %v", err)
	}

	// Verify data
	value, err := dict2.Query("numvcpus")
	if err != nil {
		t.Fatalf("Query after import failed: %v", err)
	}
	if value != "4" {
		t.Errorf("Expected '4', got '%s'", value)
	}
}

// TestDiff tests the diff function
func TestDiff(t *testing.T) {
	content1 := `key1 = "value1"
key2 = "value2"
key3 = "value3"
`

	content2 := `key1 = "value1"
key2 = "modified"
key4 = "new"
`

	file1 := createTestFile(t, content1)
	file2 := createTestFile(t, content2)
	defer os.Remove(file1)
	defer os.Remove(file2)

	dict1, _ := LoadDictionary(file1)
	dict2, _ := LoadDictionary(file2)

	diff := dict1.Diff(dict2)

	// Should show key3 removed, key2 modified, key4 added
	if !strings.Contains(diff, "key3") {
		t.Error("Expected diff to show key3 removed")
	}
	if !strings.Contains(diff, "key2") {
		t.Error("Expected diff to show key2 modified")
	}
	if !strings.Contains(diff, "key4") {
		t.Error("Expected diff to show key4 added")
	}
}

// TestGroup tests grouping entries by prefix
func TestGroup(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Group all serial0 entries
	err := dict.Group("serial0.", "")
	if err != nil {
		t.Fatalf("Group failed: %v", err)
	}

	// Find all serial0 entries and verify they're together
	var serialIndices []int
	for i, entry := range dict.Entries {
		if strings.HasPrefix(strings.ToLower(entry.Key), "serial0.") {
			serialIndices = append(serialIndices, i)
		}
	}

	if len(serialIndices) < 2 {
		t.Error("Expected at least 2 serial0 entries")
	}

	// Check they're consecutive
	for i := 1; i < len(serialIndices); i++ {
		if serialIndices[i] != serialIndices[i-1]+1 {
			t.Error("Expected serial0 entries to be consecutive after grouping")
		}
	}
}

// TestBackup tests the backup functionality
func TestBackup(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Create backup
	err := dict.Backup()
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Find backup file
	dir := filepath.Dir(filename)
	base := filepath.Base(filename)
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	foundBackup := false
	var backupFile string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), base+".backup-") {
			foundBackup = true
			backupFile = filepath.Join(dir, file.Name())
			break
		}
	}

	if !foundBackup {
		t.Error("Expected backup file to be created")
	}

	if backupFile != "" {
		defer os.Remove(backupFile)
	}
}

// TestRenameKeys tests regex-based key renaming
func TestRenameKeys(t *testing.T) {
	filename := createTestFile(t, testVMXContent)
	defer os.Remove(filename)

	dict, _ := LoadDictionary(filename)

	// Rename serial0 to uart0
	count, err := dict.RenameKeys(`serial(\d+)`, `uart$1`)
	if err != nil {
		t.Fatalf("RenameKeys failed: %v", err)
	}

	if count == 0 {
		t.Error("Expected at least one key to be renamed")
	}

	// Verify rename worked
	keys := dict.ListKeys("uart0.")
	if len(keys) < 2 {
		t.Errorf("Expected at least 2 uart0 keys after rename, got %d", len(keys))
	}

	// Verify old keys are gone
	keys = dict.ListKeys("serial0.")
	if len(keys) > 0 {
		t.Errorf("Expected no serial0 keys after rename, got %d", len(keys))
	}
}
