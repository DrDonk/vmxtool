// SPDX-FileCopyrightText: © 2025 David Parsons
// SPDX-License-Identifier: MIT
// 
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// Version information - set during build
var (
	Version   = "dev"    // version number
	BuildDate = "unknown" // build date
	Commit    = "unknown" // git commit hash
)

// Entry represents a line in the dictionary file
type Entry struct {
	Original  string // Original line including comments, whitespace
	Key       string // Extracted key (empty for comments/blank lines)
	Value     string // Extracted value (empty for comments/blank lines)
	IsComment bool   // Whether this is a comment line
	IsBlank   bool   // Whether this is a blank line
}

// Dictionary represents the file structure with preserved layout
type Dictionary struct {
	Filename string
	Entries  []*Entry
}

// LoadDictionary loads a dictionary file while preserving layout
func LoadDictionary(filename string) (*Dictionary, error) {
	dict := &Dictionary{Filename: filename}
	
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty dictionary if file doesn't exist
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
			// If it doesn't parse as key-value, treat as comment
			entry.IsComment = true
			dict.Entries = append(dict.Entries, entry)
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		
		entry.Key = key
		entry.Value = value
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
			// Preserve blank lines
			line = ""
		} else if entry.IsComment {
			// Preserve comments exactly as they were
			line = entry.Original
		} else if entry.Key != "" {
			// Rebuild key-value lines while preserving the original format as much as possible
			if strings.Contains(entry.Original, "=") {
				// Try to preserve the original formatting around the equals sign
				originalParts := strings.SplitN(entry.Original, "=", 2)
				keyPart := strings.TrimRight(originalParts[0], " \t")
				
				// Format value with quotes if needed
				formattedValue := entry.Value
				if needsQuoting(entry.Value) {
					formattedValue = `"` + entry.Value + `"`
				}
				
				line = keyPart + " = " + formattedValue
			} else {
				// Fallback if original format can't be preserved
				formattedValue := entry.Value
				if needsQuoting(entry.Value) {
					formattedValue = `"` + entry.Value + `"`
				}
				line = entry.Key + " = " + formattedValue
			}
		} else {
			// Fallback: use original line
			line = entry.Original
		}
		
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}
	
	return writer.Flush()
}

// needsQuoting checks if a value needs to be quoted
func needsQuoting(value string) bool {
	if value == "" {
		return true
	}
	
	// Check if value contains spaces, quotes, or other special characters
	for _, char := range value {
		if unicode.IsSpace(char) || char == '"' || char == '=' {
			return true
		}
	}
	return false
}

// Add adds a new key-value pair (fails if key exists)
func (d *Dictionary) Add(key, value string) error {
	if d.KeyExists(key) {
		return fmt.Errorf("key '%s' already exists", key)
	}
	
	// Add new entry at the end
	entry := &Entry{
		Original: key + " = " + value,
		Key:      key,
		Value:    value,
	}
	d.Entries = append(d.Entries, entry)
	return nil
}

// Set sets a key-value pair (adds or updates)
func (d *Dictionary) Set(key, value string) {
	// Try to update existing key
	for _, entry := range d.Entries {
		if entry.Key == key {
			entry.Value = value
			return
		}
	}
	
	// Key doesn't exist, add it at the end
	d.Add(key, value)
}

// Remove removes a key-value pair
func (d *Dictionary) Remove(key string) error {
	for i, entry := range d.Entries {
		if entry.Key == key {
			// Remove the entry
			d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("key '%s' does not exist", key)
}

// Query gets the value for a key
func (d *Dictionary) Query(key string) (string, error) {
	for _, entry := range d.Entries {
		if entry.Key == key {
			return entry.Value, nil
		}
	}
	return "", fmt.Errorf("key '%s' does not exist", key)
}

// KeyExists checks if a key exists
func (d *Dictionary) KeyExists(key string) bool {
	for _, entry := range d.Entries {
		if entry.Key == key {
			return true
		}
	}
	return false
}

// Print prints all content while preserving layout
func (d *Dictionary) Print() {
	for _, entry := range d.Entries {
		if entry.IsBlank {
			fmt.Println()
		} else if entry.IsComment {
			fmt.Println(entry.Original)
		} else if entry.Key != "" {
			formattedValue := entry.Value
			if needsQuoting(entry.Value) {
				formattedValue = `"` + entry.Value + `"`
			}
			fmt.Printf("%s = %s\n", entry.Key, formattedValue)
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
	
	// Remove quotes if present in input
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
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
	fmt.Printf("© 2025 David Parsons")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Error: no command provided")
		fmt.Println("Use 'vmxtool help' for usage information")
		os.Exit(1)
	}
	
	command := os.Args[1]
	
	switch command {
	case "help":
		printHelp()
		
	case "version":
		printVersion()
		
	case "print":
		if len(os.Args) != 3 {
			fmt.Println("Error: print command requires FILE argument")
			fmt.Println("Usage: vmxtool print FILE")
			os.Exit(1)
		}
		filename := os.Args[2]
		
		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			os.Exit(1)
		}
		
		dict.Print()
		
	case "add":
		if len(os.Args) != 4 {
			fmt.Println("Error: add command requires FILE and KEY=VALUE arguments")
			fmt.Println("Usage: vmxtool add FILE KEY=VALUE")
			os.Exit(1)
		}
		filename := os.Args[2]
		keyValue := os.Args[3]
		
		key, value, err := parseKeyValue(keyValue)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		
		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			os.Exit(1)
		}
		
		if err := dict.Add(key, value); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		
		if err := dict.Save(filename); err != nil {
			fmt.Printf("Error saving file: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Added: %s=%s\n", key, value)
		
	case "set":
		if len(os.Args) != 4 {
			fmt.Println("Error: set command requires FILE and KEY=VALUE arguments")
			fmt.Println("Usage: vmxtool set FILE KEY=VALUE")
			os.Exit(1)
		}
		filename := os.Args[2]
		keyValue := os.Args[3]
		
		key, value, err := parseKeyValue(keyValue)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		
		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			os.Exit(1)
		}
		
		dict.Set(key, value)
		
		if err := dict.Save(filename); err != nil {
			fmt.Printf("Error saving file: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Set: %s=%s\n", key, value)
		
	case "remove":
		if len(os.Args) != 4 {
			fmt.Println("Error: remove command requires FILE and KEY arguments")
			fmt.Println("Usage: vmxtool remove FILE KEY")
			os.Exit(1)
		}
		filename := os.Args[2]
		key := os.Args[3]
		
		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			os.Exit(1)
		}
		
		if err := dict.Remove(key); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		
		if err := dict.Save(filename); err != nil {
			fmt.Printf("Error saving file: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Removed: %s\n", key)
		
	case "query":
		if len(os.Args) != 4 {
			fmt.Println("Error: query command requires FILE and KEY arguments")
			fmt.Println("Usage: vmxtool query FILE KEY")
			os.Exit(1)
		}
		filename := os.Args[2]
		key := os.Args[3]
		
		dict, err := LoadDictionary(filename)
		if err != nil {
			fmt.Printf("Error loading file: %v\n", err)
			os.Exit(1)
		}
		
		value, err := dict.Query(key)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println(value)
		
	default:
		fmt.Printf("Error: unknown command '%s'\n", command)
		fmt.Println("Use 'vmxtool help' for usage information")
		os.Exit(1)
	}
}
