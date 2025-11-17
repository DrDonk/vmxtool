// SPDX-FileCopyrightText: © 2025 David Parsons
// SPDX-License-Identifier: MIT
// 
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
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
	for i, entry := range d.Entries {
		if strings.EqualFold(entry.Key, key) {
			d.Entries = slices.Delete(d.Entries, i, i+1)
			return nil
		}
	}
	return fmt.Errorf("key '%s' does not exist", key)
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

Available commands:
    help
        Prints help.

    version
        Prints version information.

    print FILE
        Prints the contents of the specified VMX file.

    add FILE KEY=VALUE
        Adds a new entry to the specified VMX file.
        Fails if the key already exists.

    set FILE KEY=VALUE
        Sets an entry in the specified VMX file, adding it if it does
        not already exist.

    remove FILE KEY
        Removes the entry with the specified key from the specified VMX
        file. Fails if the key does not exist.

    query FILE KEY
        Prints the value for the specified key from the specified VMX
        file. Fails if the key does not exist.`)
}

// printVersion displays version information
func printVersion() {
	fmt.Printf("vmxtool version %s\n", Version)
	fmt.Printf("Build date: %s\n", BuildDate)
	fmt.Printf("Commit: %s\n", Commit)
	fmt.Println("© 2025 David Parsons")
}

// run contains the main logic and returns an exit code
func run() int {
	if len(os.Args) < 2 {
		fmt.Println("Error: no command provided")
		fmt.Println("Use 'vmxtool help' for usage information")
		return 1
	}

	command := os.Args[1]

	switch command {
	case "help":
		printHelp()
		return 0

	case "version":
		printVersion()
		return 0

	case "print":
		if len(os.Args) != 3 {
			fmt.Println("Error: print command requires FILE argument")
			fmt.Println("Usage: vmxtool print FILE")
			return 1
		}
		filename := os.Args[2]

		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			return 1
		}

		dict.Print()
		return 0

	case "add":
		if len(os.Args) != 4 {
			fmt.Println("Error: add command requires FILE and KEY=VALUE arguments")
			fmt.Println("Usage: vmxtool add FILE KEY=VALUE")
			return 1
		}
		filename := os.Args[2]
		keyValue := os.Args[3]

		key, value, err := parseKeyValue(keyValue)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}

		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			return 1
		}

		if dict.KeyExists(key) {
			existingKey := dict.findEntryCaseInsensitive(key).Key
			fmt.Printf("Error: key '%s' already exists (as '%s')\n", key, existingKey)
			return 1
		}

		if err := dict.Add(key, value); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}

		if err := dict.Save(filename); err != nil {
			fmt.Printf("Error saving file: %v\n", err)
			return 1
		}

		return 0

	case "set":
		if len(os.Args) != 4 {
			fmt.Println("Error: set command requires FILE and KEY=VALUE arguments")
			fmt.Println("Usage: vmxtool set FILE KEY=VALUE")
			return 1
		}
		filename := os.Args[2]
		keyValue := os.Args[3]

		key, value, err := parseKeyValue(keyValue)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}

		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			return 1
		}

		dict.Set(key, value)

		if err := dict.Save(filename); err != nil {
			fmt.Printf("Error saving file: %v\n", err)
			return 1
		}

		return 0

	case "remove":
		if len(os.Args) != 4 {
			fmt.Println("Error: remove command requires FILE and KEY arguments")
			fmt.Println("Usage: vmxtool remove FILE KEY")
			return 1
		}
		filename := os.Args[2]
		key := os.Args[3]

		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			return 1
		}

		if err := dict.Remove(key); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}

		if err := dict.Save(filename); err != nil {
			fmt.Printf("Error saving file: %v\n", err)
			return 1
		}

		return 0

	case "query":
		if len(os.Args) != 4 {
			fmt.Println("Error: query command requires FILE and KEY arguments")
			fmt.Println("Usage: vmxtool query FILE KEY")
			return 1
		}
		filename := os.Args[2]
		key := os.Args[3]

		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			return 1
		}

		value, err := dict.Query(key)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}

		fmt.Println(value)
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
