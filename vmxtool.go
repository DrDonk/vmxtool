// SPDX-FileCopyrightText: © 2025-2026 David Parsons
// SPDX-License-Identifier: MIT
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Version information - set during build
var (
	Version   = "dev"
	BuildDate = "unknown"
	Commit    = "unknown"
)

// Entry represents a line in the dictionary file
type Entry struct {
	Original           string // Original line including comments, whitespace
	Key                string // Extracted key (empty for comments/blank lines)
	Value              string // Extracted value (empty for comments/blank lines)
	InlineComment      string // Comment text (without leading # or whitespace)
	InlineCommentSpace string // Whitespace between closing quote and # (preserved)
	IsComment          bool   // Whether this is a comment line
	IsBlank            bool   // Whether this is a blank line
}

// Dictionary represents the file structure with preserved layout
type Dictionary struct {
	Filename string
	Entries  []*Entry
}

// ExportFormat represents the structure for JSON/YAML export
type ExportFormat map[string]string

// findClosingQuote finds the index of the closing quote, handling escapes
func findClosingQuote(s string, startIdx int) int {
	for i := startIdx; i < len(s); i++ {
		if s[i] == '"' {
			// Check if it's escaped
			if i > 0 && s[i-1] == '\\' {
				continue
			}
			return i
		}
	}
	return -1
}

// LoadDictionary loads a dictionary file while preserving layout
func LoadDictionary(filename string) (*Dictionary, error) {
	dict := &Dictionary{Filename: filename}

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return dict, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		original := scanner.Text()
		trimmed := strings.TrimSpace(original)

		entry := &Entry{Original: original}

		// Check if it's a blank line
		if trimmed == "" {
			entry.IsBlank = true
			dict.Entries = append(dict.Entries, entry)
			continue
		}

		// Check if it's a comment
		if strings.HasPrefix(trimmed, "#") {
			entry.IsComment = true
			dict.Entries = append(dict.Entries, entry)
			continue
		}

		// Parse key-value pair
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			entry.IsComment = true
			dict.Entries = append(dict.Entries, entry)
			continue
		}

		key := strings.TrimSpace(parts[0])
		valueAndComment := strings.TrimSpace(parts[1])

		var value string
		var inlineComment string
		var inlineCommentSpace string

		// Handle quoted values with potential inline comments
		if strings.HasPrefix(valueAndComment, `"`) {
			// Find the closing quote
			endQuoteIdx := findClosingQuote(valueAndComment, 1)
			if endQuoteIdx != -1 {
				// Extract quoted value (without outer quotes)
				value = valueAndComment[1:endQuoteIdx]
				value = unescapeQuotes(value)

				// Everything after the closing quote
				remainder := valueAndComment[endQuoteIdx+1:]
				if len(remainder) > 0 {
					// Check if there's a comment
					if commentIdx := strings.Index(remainder, "#"); commentIdx != -1 {
						// Preserve the whitespace before #
						inlineCommentSpace = remainder[:commentIdx]
						// Store the comment (including #)
						inlineComment = remainder[commentIdx:]
					}
				}
			} else {
				// Malformed: no closing quote found, treat as unquoted
				value = valueAndComment
			}
		} else {
			// Unquoted value - check for inline comment
			if commentIdx := strings.Index(valueAndComment, "#"); commentIdx != -1 {
				value = strings.TrimSpace(valueAndComment[:commentIdx])
				// For unquoted values, preserve spacing before #
				beforeComment := valueAndComment[:commentIdx]
				if len(value) < len(beforeComment) {
					inlineCommentSpace = beforeComment[len(value):]
				}
				inlineComment = valueAndComment[commentIdx:]
			} else {
				value = valueAndComment
			}
		}

		entry.Key = key
		entry.Value = value
		entry.InlineComment = inlineComment
		entry.InlineCommentSpace = inlineCommentSpace
		dict.Entries = append(dict.Entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return dict, nil
}

// Save saves the dictionary while preserving the original layout
func (d *Dictionary) Save(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	for _, entry := range d.Entries {
		var line string

		if entry.IsBlank {
			line = ""
		} else if entry.IsComment {
			line = entry.Original
		} else if entry.Key != "" {
			// Always quote values for VMX compatibility
			formattedValue := `"` + escapeQuotes(entry.Value) + `"`

			// Rebuild key-value line
			if strings.Contains(entry.Original, "=") {
				// Try to preserve the original formatting around the equals sign
				originalParts := strings.SplitN(entry.Original, "=", 2)
				keyPart := strings.TrimRight(originalParts[0], " \t")
				line = keyPart + " = " + formattedValue
			} else {
				line = entry.Key + " = " + formattedValue
			}

			// Append inline comment with exact spacing preserved
			if entry.InlineComment != "" {
				line += entry.InlineCommentSpace + entry.InlineComment
			}
		} else {
			line = entry.Original
		}

		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// Backup creates a backup of the file
func (d *Dictionary) Backup() error {
	if d.Filename == "" {
		return errors.New("no filename specified")
	}

	// Check if original file exists
	if _, err := os.Stat(d.Filename); os.IsNotExist(err) {
		return nil // No file to backup
	}

	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("%s.backup-%s", d.Filename, timestamp)

	// Copy file
	src, err := os.Open(d.Filename)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(backupName)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	fmt.Printf("Backup created: %s\n", backupName)
	return nil
}

// escapeQuotes escapes quotes in the value
func escapeQuotes(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

// unescapeQuotes removes escape sequences from quotes
func unescapeQuotes(value string) string {
	return strings.ReplaceAll(value, `\"`, `"`)
}

// findEntryCaseInsensitive finds an entry by key (case-insensitive)
func (d *Dictionary) findEntryCaseInsensitive(key string) *Entry {
	lowerKey := strings.ToLower(key)
	for _, entry := range d.Entries {
		if strings.ToLower(entry.Key) == lowerKey {
			return entry
		}
	}
	return nil
}

// findEntryIndex finds the index of an entry by key (case-insensitive)
func (d *Dictionary) findEntryIndex(key string) int {
	lowerKey := strings.ToLower(key)
	for i, entry := range d.Entries {
		if strings.ToLower(entry.Key) == lowerKey {
			return i
		}
	}
	return -1
}

// normalizeKeyCase normalizes the key case to use the first encountered case
func (d *Dictionary) normalizeKeyCase(key string) string {
	if entry := d.findEntryCaseInsensitive(key); entry != nil {
		return entry.Key
	}
	return key
}

// Add adds a new key-value pair (fails if key exists)
func (d *Dictionary) Add(key, value string) error {
	if d.KeyExists(key) {
		return fmt.Errorf("key '%s' already exists", key)
	}

	entry := &Entry{
		Original: key + " = " + `"` + escapeQuotes(value) + `"`,
		Key:      key,
		Value:    value,
	}
	d.Entries = append(d.Entries, entry)
	return nil
}

// Set sets a key-value pair (adds or updates)
func (d *Dictionary) Set(key, value string) {
	if entry := d.findEntryCaseInsensitive(key); entry != nil {
		entry.Value = value
		// Update Original to keep it in sync, preserving inline comment
		entry.Original = entry.Key + " = " + `"` + escapeQuotes(value) + `"`
		if entry.InlineComment != "" {
			entry.Original += entry.InlineCommentSpace + entry.InlineComment
		}
		return
	}

	normalizedKey := d.normalizeKeyCase(key)
	entry := &Entry{
		Original: normalizedKey + " = " + `"` + escapeQuotes(value) + `"`,
		Key:      normalizedKey,
		Value:    value,
	}
	d.Entries = append(d.Entries, entry)
}

// Remove removes a key-value pair
func (d *Dictionary) Remove(key string) error {
	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}
	d.Entries = slices.Delete(d.Entries, idx, idx+1)
	return nil
}

// Query gets the value for a key
func (d *Dictionary) Query(key string) (string, error) {
	if entry := d.findEntryCaseInsensitive(key); entry != nil {
		return entry.Value, nil
	}
	return "", fmt.Errorf("key '%s' does not exist", key)
}

// KeyExists checks if a key exists (case-insensitive)
func (d *Dictionary) KeyExists(key string) bool {
	return d.findEntryCaseInsensitive(key) != nil
}

// AddPrefix adds a prefix to a key (modifies the key itself)
func (d *Dictionary) AddPrefix(key, prefix string) error {
	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	entry := d.Entries[idx]
	newKey := prefix + entry.Key

	// Check if the new key would conflict with an existing key (case-insensitive)
	if d.KeyExists(newKey) {
		return fmt.Errorf("key '%s' already exists (cannot add prefix)", newKey)
	}

	// Update the key
	entry.Key = newKey
	entry.Original = newKey + " = " + `"` + escapeQuotes(entry.Value) + `"`
	if entry.InlineComment != "" {
		entry.Original += entry.InlineCommentSpace + entry.InlineComment
	}

	return nil
}

// RemovePrefix removes a prefix from a key (modifies the key itself)
func (d *Dictionary) RemovePrefix(key, prefix string) error {
	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	entry := d.Entries[idx]

	// Check if the key starts with the prefix (case-insensitive)
	if !strings.HasPrefix(strings.ToLower(entry.Key), strings.ToLower(prefix)) {
		return fmt.Errorf("key '%s' does not have prefix '%s'", entry.Key, prefix)
	}

	// Remove prefix while preserving the case of the remaining part
	newKey := entry.Key[len(prefix):]

	// Check if the new key would conflict with an existing key (case-insensitive)
	if d.KeyExists(newKey) {
		return fmt.Errorf("key '%s' already exists (cannot remove prefix)", newKey)
	}

	// Update the key
	entry.Key = newKey
	entry.Original = newKey + " = " + `"` + escapeQuotes(entry.Value) + `"`
	if entry.InlineComment != "" {
		entry.Original += entry.InlineCommentSpace + entry.InlineComment
	}

	return nil
}

// InsertCommentBefore inserts a comment line before the specified key
func (d *Dictionary) InsertCommentBefore(key, comment string) error {
	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	// Ensure comment starts with #
	if !strings.HasPrefix(comment, "#") {
		comment = "# " + comment
	}

	commentEntry := &Entry{
		Original:  comment,
		IsComment: true,
	}

	// Insert before the found entry
	d.Entries = slices.Insert(d.Entries, idx, commentEntry)
	return nil
}

// InsertCommentAfter inserts a comment line after the specified key
func (d *Dictionary) InsertCommentAfter(key, comment string) error {
	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	// Ensure comment starts with #
	if !strings.HasPrefix(comment, "#") {
		comment = "# " + comment
	}

	commentEntry := &Entry{
		Original:  comment,
		IsComment: true,
	}

	// Insert after the found entry
	d.Entries = slices.Insert(d.Entries, idx+1, commentEntry)
	return nil
}

// InsertBlankLinesBefore inserts one or more blank lines before the specified key
func (d *Dictionary) InsertBlankLinesBefore(key string, count int) error {
	if count < 1 {
		return errors.New("count must be at least 1")
	}

	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	// Create blank line entries
	blankEntries := make([]*Entry, count)
	for i := 0; i < count; i++ {
		blankEntries[i] = &Entry{
			Original: "",
			IsBlank:  true,
		}
	}

	// Insert before the found entry
	d.Entries = slices.Insert(d.Entries, idx, blankEntries...)
	return nil
}

// InsertBlankLinesAfter inserts one or more blank lines after the specified key
func (d *Dictionary) InsertBlankLinesAfter(key string, count int) error {
	if count < 1 {
		return errors.New("count must be at least 1")
	}

	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	// Create blank line entries
	blankEntries := make([]*Entry, count)
	for i := 0; i < count; i++ {
		blankEntries[i] = &Entry{
			Original: "",
			IsBlank:  true,
		}
	}

	// Insert after the found entry
	d.Entries = slices.Insert(d.Entries, idx+1, blankEntries...)
	return nil
}

// RemoveCommentBefore removes a comment line immediately before the specified key
func (d *Dictionary) RemoveCommentBefore(key string) error {
	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	if idx == 0 {
		return errors.New("no comment before this key")
	}

	// Check if the previous entry is a comment
	if d.Entries[idx-1].IsComment {
		d.Entries = slices.Delete(d.Entries, idx-1, idx)
		return nil
	}

	return errors.New("no comment found before this key")
}

// RemoveCommentAfter removes a comment line immediately after the specified key
func (d *Dictionary) RemoveCommentAfter(key string) error {
	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	if idx >= len(d.Entries)-1 {
		return errors.New("no comment after this key")
	}

	// Check if the next entry is a comment
	if d.Entries[idx+1].IsComment {
		d.Entries = slices.Delete(d.Entries, idx+1, idx+2)
		return nil
	}

	return errors.New("no comment found after this key")
}

// RemoveBlankBefore removes blank line(s) immediately before the specified key
func (d *Dictionary) RemoveBlankBefore(key string, count int) error {
	if count < 1 {
		return errors.New("count must be at least 1")
	}

	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	removed := 0
	for i := idx - 1; i >= 0 && removed < count; i-- {
		if d.Entries[i].IsBlank {
			d.Entries = slices.Delete(d.Entries, i, i+1)
			removed++
			idx-- // Adjust index as we're removing entries
		} else {
			break // Stop if we hit a non-blank line
		}
	}

	if removed == 0 {
		return errors.New("no blank lines found before this key")
	}

	return nil
}

// RemoveBlankAfter removes blank line(s) immediately after the specified key
func (d *Dictionary) RemoveBlankAfter(key string, count int) error {
	if count < 1 {
		return errors.New("count must be at least 1")
	}

	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	removed := 0
	for i := idx + 1; i < len(d.Entries) && removed < count; {
		if d.Entries[i].IsBlank {
			d.Entries = slices.Delete(d.Entries, i, i+1)
			removed++
			// Don't increment i since we deleted an entry
		} else {
			break // Stop if we hit a non-blank line
		}
	}

	if removed == 0 {
		return errors.New("no blank lines found after this key")
	}

	return nil
}

// RemoveAllComments removes all comment lines from the dictionary
func (d *Dictionary) RemoveAllComments() int {
	count := 0
	i := 0
	for i < len(d.Entries) {
		if d.Entries[i].IsComment {
			d.Entries = slices.Delete(d.Entries, i, i+1)
			count++
		} else {
			i++
		}
	}
	return count
}

// RemoveAllBlankLines removes all blank lines from the dictionary
func (d *Dictionary) RemoveAllBlankLines() int {
	count := 0
	i := 0
	for i < len(d.Entries) {
		if d.Entries[i].IsBlank {
			d.Entries = slices.Delete(d.Entries, i, i+1)
			count++
		} else {
			i++
		}
	}
	return count
}

// ListKeys returns all keys in the dictionary, optionally filtered by prefix
func (d *Dictionary) ListKeys(prefix string) []string {
	var keys []string
	lowerPrefix := strings.ToLower(prefix)
	for _, entry := range d.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			if prefix == "" || strings.HasPrefix(strings.ToLower(entry.Key), lowerPrefix) {
				keys = append(keys, entry.Key)
			}
		}
	}
	return keys
}

// Search finds all keys or values matching a pattern
func (d *Dictionary) Search(pattern string, searchValues bool) ([]string, error) {
	re, err := regexp.Compile("(?i)" + pattern) // Case-insensitive
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %v", err)
	}

	var results []string
	for _, entry := range d.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			if searchValues {
				if re.MatchString(entry.Value) {
					results = append(results, fmt.Sprintf("%s = \"%s\"", entry.Key, entry.Value))
				}
			} else {
				if re.MatchString(entry.Key) {
					results = append(results, entry.Key)
				}
			}
		}
	}
	return results, nil
}

// Sort sorts all key-value entries alphabetically while preserving sections
func (d *Dictionary) Sort(preserveSections bool) {
	if !preserveSections {
		// Simple sort: extract all key-value entries, sort them, rebuild
		var kvEntries []*Entry
		var otherEntries []*Entry

		for _, entry := range d.Entries {
			if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
				kvEntries = append(kvEntries, entry)
			} else {
				otherEntries = append(otherEntries, entry)
			}
		}

		sort.Slice(kvEntries, func(i, j int) bool {
			return strings.ToLower(kvEntries[i].Key) < strings.ToLower(kvEntries[j].Key)
		})

		// Rebuild: comments/blanks first, then sorted entries
		d.Entries = append(otherEntries, kvEntries...)
	} else {
		// Sort within sections (sections are separated by blank lines or comment blocks)
		var sections [][]*Entry
		currentSection := []*Entry{}

		for _, entry := range d.Entries {
			if entry.IsBlank || entry.IsComment {
				// End current section
				if len(currentSection) > 0 {
					sections = append(sections, currentSection)
					currentSection = []*Entry{}
				}
				sections = append(sections, []*Entry{entry})
			} else {
				currentSection = append(currentSection, entry)
			}
		}
		if len(currentSection) > 0 {
			sections = append(sections, currentSection)
		}

		// Sort each section that contains key-value pairs
		for _, section := range sections {
			if len(section) > 0 && section[0].Key != "" {
				sort.Slice(section, func(i, j int) bool {
					return strings.ToLower(section[i].Key) < strings.ToLower(section[j].Key)
				})
			}
		}

		// Flatten sections back
		d.Entries = []*Entry{}
		for _, section := range sections {
			d.Entries = append(d.Entries, section...)
		}
	}
}

// Group moves all entries with a given prefix together
func (d *Dictionary) Group(prefix string, afterKey string) error {
	lowerPrefix := strings.ToLower(prefix)

	// Find all entries with the prefix (case-insensitive)
	var matchingEntries []*Entry
	var remainingEntries []*Entry

	for _, entry := range d.Entries {
		if entry.Key != "" && strings.HasPrefix(strings.ToLower(entry.Key), lowerPrefix) {
			matchingEntries = append(matchingEntries, entry)
		} else {
			remainingEntries = append(remainingEntries, entry)
		}
	}

	if len(matchingEntries) == 0 {
		return fmt.Errorf("no keys found with prefix '%s'", prefix)
	}

	// Find insertion point
	insertIdx := len(remainingEntries)
	if afterKey != "" {
		for i, entry := range remainingEntries {
			if strings.EqualFold(entry.Key, afterKey) {
				insertIdx = i + 1
				break
			}
		}
	}

	// Rebuild entries
	d.Entries = slices.Insert(remainingEntries, insertIdx, matchingEntries...)
	return nil
}

// AddSection adds a formatted section header before a key
func (d *Dictionary) AddSection(key, sectionName string) error {
	idx := d.findEntryIndex(key)
	if idx == -1 {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	// Create section header
	header := fmt.Sprintf("# >>> %s <<<", sectionName)
	commentEntry := &Entry{
		Original:  header,
		IsComment: true,
	}

	// Insert before the found entry
	d.Entries = slices.Insert(d.Entries, idx, commentEntry)
	return nil
}

// RenameKeys renames keys using regex pattern replacement
func (d *Dictionary) RenameKeys(pattern, replacement string) (int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, fmt.Errorf("invalid pattern: %v", err)
	}

	count := 0
	for _, entry := range d.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			newKey := re.ReplaceAllString(entry.Key, replacement)
			if newKey != entry.Key {
				// Check for conflicts
				if d.KeyExists(newKey) && !strings.EqualFold(entry.Key, newKey) {
					return count, fmt.Errorf("rename would create duplicate key: %s", newKey)
				}
				entry.Key = newKey
				entry.Original = newKey + " = " + `"` + escapeQuotes(entry.Value) + `"`
				if entry.InlineComment != "" {
					entry.Original += entry.InlineCommentSpace + entry.InlineComment
				}
				count++
			}
		}
	}

	return count, nil
}

// RemoveByPrefix removes all entries with a given prefix
func (d *Dictionary) RemoveByPrefix(prefix string) (int, error) {
	lowerPrefix := strings.ToLower(prefix)
	count := 0
	i := 0
	for i < len(d.Entries) {
		entry := d.Entries[i]
		if entry.Key != "" && strings.HasPrefix(strings.ToLower(entry.Key), lowerPrefix) {
			d.Entries = slices.Delete(d.Entries, i, i+1)
			count++
		} else {
			i++
		}
	}

	if count == 0 {
		return 0, fmt.Errorf("no keys found with prefix '%s'", prefix)
	}

	return count, nil
}

// CommentOut converts a key-value entry to a comment
func (d *Dictionary) CommentOut(key string) error {
	entry := d.findEntryCaseInsensitive(key)
	if entry == nil {
		return fmt.Errorf("key '%s' does not exist", key)
	}

	// Convert to comment
	entry.IsComment = true
	entry.Original = "# " + entry.Original
	entry.Key = ""
	entry.Value = ""

	return nil
}

// Uncomment converts a commented key-value entry back to active
func (d *Dictionary) Uncomment(key string) error {
	lowerKey := strings.ToLower(key)

	// Find a commented line that contains the key
	for _, entry := range d.Entries {
		if entry.IsComment {
			// Remove all leading # characters and whitespace
			uncommented := entry.Original
			for strings.HasPrefix(strings.TrimSpace(uncommented), "#") {
				uncommented = strings.TrimSpace(uncommented)
				uncommented = strings.TrimPrefix(uncommented, "#")
			}
			uncommented = strings.TrimSpace(uncommented)

			// Parse the uncommented line
			parts := strings.SplitN(uncommented, "=", 2)
			if len(parts) == 2 {
				parsedKey := strings.TrimSpace(parts[0])
				// Case-insensitive comparison
				if strings.ToLower(parsedKey) == lowerKey {
					// Re-parse as a key-value entry
					entry.IsComment = false
					entry.Key = parsedKey

					valueAndComment := strings.TrimSpace(parts[1])
					var value string
					var inlineComment string
					var inlineCommentSpace string

					// Handle quoted values with potential inline comments
					if strings.HasPrefix(valueAndComment, `"`) {
						endQuoteIdx := findClosingQuote(valueAndComment, 1)
						if endQuoteIdx != -1 {
							value = valueAndComment[1:endQuoteIdx]
							value = unescapeQuotes(value)

							remainder := valueAndComment[endQuoteIdx+1:]
							if len(remainder) > 0 {
								if commentIdx := strings.Index(remainder, "#"); commentIdx != -1 {
									inlineCommentSpace = remainder[:commentIdx]
									inlineComment = remainder[commentIdx:]
								}
							}
						} else {
							value = valueAndComment
						}
					} else {
						if commentIdx := strings.Index(valueAndComment, "#"); commentIdx != -1 {
							value = strings.TrimSpace(valueAndComment[:commentIdx])
							beforeComment := valueAndComment[:commentIdx]
							if len(value) < len(beforeComment) {
								inlineCommentSpace = beforeComment[len(value):]
							}
							inlineComment = valueAndComment[commentIdx:]
						} else {
							value = valueAndComment
						}
					}

					entry.Value = value
					entry.InlineComment = inlineComment
					entry.InlineCommentSpace = inlineCommentSpace
					entry.Original = entry.Key + " = " + `"` + escapeQuotes(entry.Value) + `"`
					if entry.InlineComment != "" {
						entry.Original += entry.InlineCommentSpace + entry.InlineComment
					}

					return nil
				}
			}
		}
	}

	return fmt.Errorf("commented key '%s' not found", key)
}

// ExportToJSON exports all key-value pairs to JSON
func (d *Dictionary) ExportToJSON() (string, error) {
	data := make(ExportFormat)
	for _, entry := range d.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			data[entry.Key] = entry.Value
		}
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// ExportToYAML exports all key-value pairs to YAML
func (d *Dictionary) ExportToYAML() (string, error) {
	data := make(ExportFormat)
	for _, entry := range d.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			data[entry.Key] = entry.Value
		}
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
}

// ImportFromJSON imports key-value pairs from JSON
func (d *Dictionary) ImportFromJSON(jsonData string) error {
	var data ExportFormat
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}

	for key, value := range data {
		d.Set(key, value)
	}

	return nil
}

// ImportFromYAML imports key-value pairs from YAML
func (d *Dictionary) ImportFromYAML(yamlData string) error {
	var data ExportFormat
	if err := yaml.Unmarshal([]byte(yamlData), &data); err != nil {
		return fmt.Errorf("invalid YAML: %v", err)
	}

	for key, value := range data {
		d.Set(key, value)
	}

	return nil
}

// ApplyTemplate applies a template with variable substitution
func (d *Dictionary) ApplyTemplate(templateDict *Dictionary, variables map[string]string) error {
	for _, entry := range templateDict.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			// Substitute variables in both key and value
			key := entry.Key
			value := entry.Value

			for varName, varValue := range variables {
				placeholder := "${" + varName + "}"
				key = strings.ReplaceAll(key, placeholder, varValue)
				value = strings.ReplaceAll(value, placeholder, varValue)
			}

			d.Set(key, value)
		}
	}

	return nil
}

// Extract creates a new dictionary with only entries matching a pattern
func (d *Dictionary) Extract(pattern string) (*Dictionary, error) {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %v", err)
	}

	newDict := &Dictionary{Filename: d.Filename}
	for _, entry := range d.Entries {
		if entry.Key != "" && re.MatchString(entry.Key) {
			newDict.Entries = append(newDict.Entries, entry)
		}
	}

	return newDict, nil
}

// Merge merges another dictionary into this one
func (d *Dictionary) Merge(other *Dictionary, overwrite bool) error {
	for _, entry := range other.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			if overwrite || !d.KeyExists(entry.Key) {
				d.Set(entry.Key, entry.Value)
			}
		}
	}

	return nil
}

// Diff compares two dictionaries and returns differences
func (d *Dictionary) Diff(other *Dictionary) string {
	var diff strings.Builder

	// Find keys only in first dict
	for _, entry := range d.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			if !other.KeyExists(entry.Key) {
				diff.WriteString(fmt.Sprintf("- %s = \"%s\"\n", entry.Key, entry.Value))
			} else {
				otherValue, _ := other.Query(entry.Key)
				if entry.Value != otherValue {
					diff.WriteString(fmt.Sprintf("< %s = \"%s\"\n", entry.Key, entry.Value))
					diff.WriteString(fmt.Sprintf("> %s = \"%s\"\n", entry.Key, otherValue))
				}
			}
		}
	}

	// Find keys only in second dict
	for _, entry := range other.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			if !d.KeyExists(entry.Key) {
				diff.WriteString(fmt.Sprintf("+ %s = \"%s\"\n", entry.Key, entry.Value))
			}
		}
	}

	return diff.String()
}

// Validate checks for common issues
func (d *Dictionary) Validate() []string {
	var issues []string
	seen := make(map[string]bool)

	for _, entry := range d.Entries {
		if entry.Key != "" && !entry.IsComment && !entry.IsBlank {
			lowerKey := strings.ToLower(entry.Key)
			if seen[lowerKey] {
				issues = append(issues, fmt.Sprintf("Duplicate key: %s", entry.Key))
			}
			seen[lowerKey] = true

			// Check for empty values
			if entry.Value == "" {
				issues = append(issues, fmt.Sprintf("Empty value for key: %s", entry.Key))
			}

			// Check for unescaped quotes in values
			if strings.Contains(entry.Value, `"`) && !strings.Contains(entry.Value, `\"`) {
				issues = append(issues, fmt.Sprintf("Unescaped quote in value: %s", entry.Key))
			}
		}
	}

	return issues
}

// BulkSet sets multiple key-value pairs from a reader
func (d *Dictionary) BulkSet(reader io.Reader) (int, error) {
	count := 0
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return count, fmt.Errorf("invalid format in bulk set: %s", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
			value = unescapeQuotes(value)
		}

		d.Set(key, value)
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, err
	}

	return count, nil
}

// Print prints all content while preserving layout
func (d *Dictionary) Print() {
	for _, entry := range d.Entries {
		if entry.IsBlank {
			fmt.Println()
		} else if entry.IsComment {
			fmt.Println(entry.Original)
		} else if entry.Key != "" {
			formattedValue := `"` + escapeQuotes(entry.Value) + `"`
			line := fmt.Sprintf("%s = %s", entry.Key, formattedValue)
			if entry.InlineComment != "" {
				line += entry.InlineCommentSpace + entry.InlineComment
			}
			fmt.Println(line)
		} else {
			fmt.Println(entry.Original)
		}
	}
}

// parseKeyValue parses a KEY=VALUE string
func parseKeyValue(kv string) (string, string, error) {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return "", "", errors.New("invalid format: expected KEY=VALUE")
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Remove quotes if present in input and unescape
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
		value = unescapeQuotes(value)
	}

	if key == "" {
		return "", "", errors.New("key cannot be empty")
	}

	return key, value, nil
}

// printHelp displays the help message
func printHelp() {
	fmt.Println(`A tool to examine and modify VMware VMX configuration files.

BASIC COMMANDS:
    help                    Print this help message
    version                 Print version information
    print FILE              Print the contents of FILE
    query FILE KEY          Print the value for KEY
    add FILE KEY=VALUE      Add a new entry (fails if exists)
    set FILE KEY=VALUE      Set an entry (adds or updates)
    remove FILE KEY         Remove an entry

PREFIX OPERATIONS:
    add-prefix FILE KEY PREFIX
        Add PREFIX to the beginning of KEY

    remove-prefix FILE KEY PREFIX
        Remove PREFIX from the beginning of KEY

    remove-prefix-all FILE PREFIX
        Remove all entries with PREFIX

INSERTION COMMANDS:
    insert-comment-before FILE KEY COMMENT
        Insert a comment line before KEY

    insert-comment-after FILE KEY COMMENT
        Insert a comment line after KEY

    insert-blank-before FILE KEY [COUNT]
        Insert blank line(s) before KEY (default: 1)

    insert-blank-after FILE KEY [COUNT]
        Insert blank line(s) after KEY (default: 1)

REMOVAL COMMANDS:
    remove-comment-before FILE KEY
        Remove comment line immediately before KEY

    remove-comment-after FILE KEY
        Remove comment line immediately after KEY

    remove-blank-before FILE KEY [COUNT]
        Remove blank line(s) immediately before KEY (default: 1)

    remove-blank-after FILE KEY [COUNT]
        Remove blank line(s) immediately after KEY (default: 1)

    remove-all-comments FILE
        Remove all comment lines from the file

    remove-all-blanks FILE
        Remove all blank lines from the file

ORGANIZATION:
    sort FILE [--preserve-sections]
        Sort entries alphabetically

    group FILE PREFIX [--after KEY]
        Group all entries with PREFIX together

    add-section FILE KEY "SECTION NAME"
        Add a formatted section header before KEY

SEARCH & QUERY:
    list-keys FILE [--prefix PREFIX]
        List all keys, optionally filtered by prefix

    search FILE PATTERN [--values]
        Search for keys (or values with --values) matching PATTERN

    diff FILE1 FILE2
        Show differences between two files

    validate FILE
        Check for common issues (duplicates, empty values, etc.)

BULK OPERATIONS:
    rename FILE PATTERN REPLACEMENT
        Rename keys using regex (e.g., "serial(\d+)" "uart\1")

    comment FILE KEY
        Comment out an entry (disable without deleting)

    uncomment FILE KEY
        Uncomment a previously commented entry

    bulk-set FILE --from SOURCE
        Set multiple entries from SOURCE file or stdin (-)

IMPORT/EXPORT:
    export FILE --format FORMAT
        Export to json or yaml

    import FILE SOURCE [--format FORMAT]
        Import from json or yaml file

    extract FILE PATTERN --output OUTPUT
        Extract entries matching PATTERN to OUTPUT

    merge FILE1 FILE2 [--output OUTPUT] [--overwrite]
        Merge FILE2 into FILE1

    apply-template FILE TEMPLATE --var NAME=VALUE ...
        Apply TEMPLATE with variable substitution

OPTIONS:
    --backup                Create backup before modifying (for set/add/remove)`)
}

// printVersion displays version information
func printVersion() {
	fmt.Printf("vmxtool version %s\n", Version)
	fmt.Printf("Build date: %s\n", BuildDate)
	fmt.Printf("Commit: %s\n", Commit)
	fmt.Println("© 2025 David Parsons")
}

// parseFlags extracts flags from arguments
func parseFlags(args []string) ([]string, map[string]string) {
	var nonFlags []string
	flags := make(map[string]string)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			flagName := strings.TrimPrefix(arg, "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				flags[flagName] = args[i+1]
				i++ // Skip next arg
			} else {
				flags[flagName] = "true"
			}
		} else {
			nonFlags = append(nonFlags, arg)
		}
	}

	return nonFlags, flags
}

// run contains the main logic and returns an exit code
func run() int {
	if len(os.Args) < 2 {
		fmt.Println("Error: no command provided")
		fmt.Println("Use 'vmxtool help' for usage information")
		return 1
	}

	args, flags := parseFlags(os.Args[1:])
	if len(args) == 0 {
		fmt.Println("Error: no command provided")
		return 1
	}

	command := args[0]
	shouldBackup := flags["backup"] == "true"

	switch command {
	case "help":
		printHelp()
		return 0

	case "version":
		printVersion()
		return 0

	case "print":
		if len(args) != 2 {
			fmt.Println("Error: print command requires FILE argument")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		dict.Print()
		return 0

	case "query":
		if len(args) != 3 {
			fmt.Println("Error: query command requires FILE and KEY arguments")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		value, err := dict.Query(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		fmt.Println(value)
		return 0

	case "add":
		if len(args) != 3 {
			fmt.Println("Error: add command requires FILE and KEY=VALUE arguments")
			return 1
		}
		key, value, err := parseKeyValue(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.Add(key, value); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "set":
		if len(args) != 3 {
			fmt.Println("Error: set command requires FILE and KEY=VALUE arguments")
			return 1
		}
		key, value, err := parseKeyValue(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		dict.Set(key, value)
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove":
		if len(args) != 3 {
			fmt.Println("Error: remove command requires FILE and KEY arguments")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.Remove(args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "add-prefix":
		if len(args) != 4 {
			fmt.Println("Error: add-prefix requires FILE KEY PREFIX")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.AddPrefix(args[2], args[3]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove-prefix":
		if len(args) != 4 {
			fmt.Println("Error: remove-prefix requires FILE KEY PREFIX")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.RemovePrefix(args[2], args[3]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove-prefix-all":
		if len(args) != 3 {
			fmt.Println("Error: remove-prefix-all requires FILE PREFIX")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		count, err := dict.RemoveByPrefix(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		fmt.Printf("Removed %d entries\n", count)
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "insert-comment-before":
		if len(args) != 4 {
			fmt.Println("Error: insert-comment-before requires FILE KEY COMMENT")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.InsertCommentBefore(args[2], args[3]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "insert-comment-after":
		if len(args) != 4 {
			fmt.Println("Error: insert-comment-after requires FILE KEY COMMENT")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.InsertCommentAfter(args[2], args[3]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "insert-blank-before":
		if len(args) < 3 {
			fmt.Println("Error: insert-blank-before requires FILE KEY [COUNT]")
			return 1
		}
		count := 1
		if len(args) >= 4 {
			var err error
			count, err = strconv.Atoi(args[3])
			if err != nil {
				fmt.Printf("Error: invalid count: %v\n", err)
				return 1
			}
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.InsertBlankLinesBefore(args[2], count); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "insert-blank-after":
		if len(args) < 3 {
			fmt.Println("Error: insert-blank-after requires FILE KEY [COUNT]")
			return 1
		}
		count := 1
		if len(args) >= 4 {
			var err error
			count, err = strconv.Atoi(args[3])
			if err != nil {
				fmt.Printf("Error: invalid count: %v\n", err)
				return 1
			}
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.InsertBlankLinesAfter(args[2], count); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove-comment-before":
		if len(args) != 3 {
			fmt.Println("Error: remove-comment-before requires FILE KEY")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.RemoveCommentBefore(args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove-comment-after":
		if len(args) != 3 {
			fmt.Println("Error: remove-comment-after requires FILE KEY")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.RemoveCommentAfter(args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove-blank-before":
		if len(args) < 3 {
			fmt.Println("Error: remove-blank-before requires FILE KEY [COUNT]")
			return 1
		}
		count := 1
		if len(args) >= 4 {
			var err error
			count, err = strconv.Atoi(args[3])
			if err != nil {
				fmt.Printf("Error: invalid count: %v\n", err)
				return 1
			}
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.RemoveBlankBefore(args[2], count); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove-blank-after":
		if len(args) < 3 {
			fmt.Println("Error: remove-blank-after requires FILE KEY [COUNT]")
			return 1
		}
		count := 1
		if len(args) >= 4 {
			var err error
			count, err = strconv.Atoi(args[3])
			if err != nil {
				fmt.Printf("Error: invalid count: %v\n", err)
				return 1
			}
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.RemoveBlankAfter(args[2], count); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove-all-comments":
		if len(args) != 2 {
			fmt.Println("Error: remove-all-comments requires FILE")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		count := dict.RemoveAllComments()
		fmt.Printf("Removed %d comment lines\n", count)
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "remove-all-blanks":
		if len(args) != 2 {
			fmt.Println("Error: remove-all-blanks requires FILE")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		count := dict.RemoveAllBlankLines()
		fmt.Printf("Removed %d blank lines\n", count)
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "list-keys":
		if len(args) < 2 {
			fmt.Println("Error: list-keys requires FILE")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		prefix := flags["prefix"]
		keys := dict.ListKeys(prefix)
		for _, key := range keys {
			fmt.Println(key)
		}
		return 0

	case "search":
		if len(args) < 3 {
			fmt.Println("Error: search requires FILE PATTERN")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		searchValues := flags["values"] == "true"
		results, err := dict.Search(args[2], searchValues)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		for _, result := range results {
			fmt.Println(result)
		}
		return 0

	case "sort":
		if len(args) < 2 {
			fmt.Println("Error: sort requires FILE")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		preserveSections := flags["preserve-sections"] == "true"
		dict.Sort(preserveSections)
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "group":
		if len(args) < 3 {
			fmt.Println("Error: group requires FILE PREFIX")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		afterKey := flags["after"]
		if err := dict.Group(args[2], afterKey); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "add-section":
		if len(args) != 4 {
			fmt.Println("Error: add-section requires FILE KEY SECTION_NAME")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.AddSection(args[2], args[3]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "rename":
		if len(args) != 4 {
			fmt.Println("Error: rename requires FILE PATTERN REPLACEMENT")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		count, err := dict.RenameKeys(args[2], args[3])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		fmt.Printf("Renamed %d keys\n", count)
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "comment":
		if len(args) != 3 {
			fmt.Println("Error: comment requires FILE KEY")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.CommentOut(args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "uncomment":
		if len(args) != 3 {
			fmt.Println("Error: uncomment requires FILE KEY")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		if err := dict.Uncomment(args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "bulk-set":
		if len(args) < 2 {
			fmt.Println("Error: bulk-set requires FILE --from SOURCE")
			return 1
		}
		source := flags["from"]
		if source == "" {
			fmt.Println("Error: bulk-set requires --from flag")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		var reader io.Reader
		if source == "-" {
			reader = os.Stdin
		} else {
			file, err := os.Open(source)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return 1
			}
			defer file.Close()
			reader = file
		}
		count, err := dict.BulkSet(reader)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		fmt.Printf("Set %d entries\n", count)
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "export":
		if len(args) < 2 {
			fmt.Println("Error: export requires FILE --format FORMAT")
			return 1
		}
		format := flags["format"]
		if format == "" {
			format = "json"
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		var output string
		switch format {
		case "json":
			output, err = dict.ExportToJSON()
		case "yaml":
			output, err = dict.ExportToYAML()
		default:
			fmt.Printf("Error: unsupported format '%s'\n", format)
			return 1
		}
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		fmt.Println(output)
		return 0

	case "import":
		if len(args) < 3 {
			fmt.Println("Error: import requires FILE SOURCE")
			return 1
		}
		format := flags["format"]
		if format == "" {
			// Auto-detect from extension
			ext := filepath.Ext(args[2])
			if ext == ".json" {
				format = "json"
			} else if ext == ".yaml" || ext == ".yml" {
				format = "yaml"
			} else {
				fmt.Println("Error: cannot detect format, use --format flag")
				return 1
			}
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		data, err := os.ReadFile(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		switch format {
		case "json":
			err = dict.ImportFromJSON(string(data))
		case "yaml":
			err = dict.ImportFromYAML(string(data))
		default:
			fmt.Printf("Error: unsupported format '%s'\n", format)
			return 1
		}
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "extract":
		if len(args) < 3 {
			fmt.Println("Error: extract requires FILE PATTERN --output OUTPUT")
			return 1
		}
		output := flags["output"]
		if output == "" {
			fmt.Println("Error: extract requires --output flag")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		extracted, err := dict.Extract(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			// Backup the output file if it exists
			if _, err := os.Stat(output); err == nil {
				extracted.Filename = output
				if err := extracted.Backup(); err != nil {
					fmt.Printf("Error creating backup: %v\n", err)
					return 1
				}
			}
		}
		if err := extracted.Save(output); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "merge":
		if len(args) < 3 {
			fmt.Println("Error: merge requires FILE1 FILE2")
			return 1
		}
		dict1, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		dict2, err := LoadDictionary(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict1.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		overwrite := flags["overwrite"] == "true"
		if err := dict1.Merge(dict2, overwrite); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		output := flags["output"]
		if output == "" {
			output = args[1]
		}
		if err := dict1.Save(output); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	case "diff":
		if len(args) < 3 {
			fmt.Println("Error: diff requires FILE1 FILE2")
			return 1
		}
		dict1, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		dict2, err := LoadDictionary(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		diff := dict1.Diff(dict2)
		if diff == "" {
			fmt.Println("No differences found")
		} else {
			fmt.Print(diff)
		}
		return 0

	case "validate":
		if len(args) < 2 {
			fmt.Println("Error: validate requires FILE")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		issues := dict.Validate()
		if len(issues) == 0 {
			fmt.Println("No issues found")
		} else {
			for _, issue := range issues {
				fmt.Println(issue)
			}
		}
		return 0

	case "apply-template":
		if len(args) < 3 {
			fmt.Println("Error: apply-template requires FILE TEMPLATE --var NAME=VALUE")
			return 1
		}
		dict, err := LoadDictionary(args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if shouldBackup {
			if err := dict.Backup(); err != nil {
				fmt.Printf("Error creating backup: %v\n", err)
				return 1
			}
		}
		template, err := LoadDictionary(args[2])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		// Parse variables from --var flags
		variables := make(map[string]string)
		varFlag := flags["var"]
		if varFlag != "" {
			parts := strings.SplitN(varFlag, "=", 2)
			if len(parts) == 2 {
				variables[parts[0]] = parts[1]
			}
		}
		if err := dict.ApplyTemplate(template, variables); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		if err := dict.Save(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0

	default:
		fmt.Printf("Error: unknown command '%s'\n", command)
		fmt.Println("Use 'vmxtool help' for usage information")
		return 1
	}
}

func main() {
	os.Exit(run())
}
