//go:build ignore
// +build ignore

// SPDX-FileCopyrightText: © 2025 David Parsons
// SPDX-License-Identifier: MIT
//
// Task runner for vmxtool - works on macOS, Linux, and Windows
// Usage: go run tasks.go [command]

package main

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	buildDir = "build"
	distDir  = "dist"
)

// BuildTarget represents a build configuration
type BuildTarget struct {
	OS   string
	Arch string
}

var targets = []BuildTarget{
	{"windows", "amd64"},
	{"windows", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "help", "-h", "--help":
		printHelp()
	case "version", "-v", "--version":
		showVersion()
	case "test":
		runTests()
	case "build":
		buildCurrent()
	case "build-all":
		buildAll()
	case "dist":
		createDist()
	case "clean":
		clean()
	case "all":
		runAll()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'go run tasks.go help' for usage")
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("vmxtool task runner")
	fmt.Println("")
	fmt.Println("Usage: go run tasks.go [command]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  help       - Show this help message")
	fmt.Println("  version    - Show version information")
	fmt.Println("  test       - Run tests")
	fmt.Println("  build      - Build for current platform")
	fmt.Println("  build-all  - Build for all platforms")
	fmt.Println("  dist       - Create distribution archive")
	fmt.Println("  clean      - Remove build artifacts")
	fmt.Println("  all        - Run tests, build all, and create dist (default)")
	fmt.Println("")
	version, _, _ := getVersionInfo()
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func showVersion() {
	version, buildDate, commit := getVersionInfo()
	fmt.Printf("Version:    %s\n", version)
	fmt.Printf("Build Date: %s\n", buildDate)
	fmt.Printf("Commit:     %s\n", commit)
	fmt.Printf("Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func getVersionInfo() (version, buildDate, commit string) {
	// Read version from VERSION file
	versionBytes, err := os.ReadFile("VERSION")
	if err != nil {
		version = "dev"
	} else {
		version = strings.TrimSpace(string(versionBytes))
	}

	// Get build date
	buildDate = time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// Get git commit
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		commit = "unknown"
	} else {
		commit = strings.TrimSpace(string(output))
	}

	return
}

func runTests() {
	fmt.Println("Running tests...")
	cmd := exec.Command("go", "test", "-v")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Tests failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Tests passed!")
}

func buildCurrent() {
	fmt.Printf("Building for %s/%s...\n", runtime.GOOS, runtime.GOARCH)

	version, buildDate, commit := getVersionInfo()
	ldflags := fmt.Sprintf("-X main.Version=%s -X main.BuildDate=%s -X main.Commit=%s",
		version, buildDate, commit)

	output := "vmxtool"
	if runtime.GOOS == "windows" {
		output = "vmxtool.exe"
	}

	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", output)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Built: %s\n", output)
}

func buildAll() {
	version, buildDate, commit := getVersionInfo()
	ldflags := fmt.Sprintf("-X main.Version=%s -X main.BuildDate=%s -X main.Commit=%s",
		version, buildDate, commit)

	// Build for all targets
	for _, target := range targets {
		fmt.Printf("Building %s/%s...\n", target.OS, target.Arch)

		outputDir := filepath.Join(buildDir, target.OS, target.Arch)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("Failed to create directory %s: %v\n", outputDir, err)
			os.Exit(1)
		}

		output := filepath.Join(outputDir, "vmxtool")
		if target.OS == "windows" {
			output += ".exe"
		}

		cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", output)
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("GOOS=%s", target.OS),
			fmt.Sprintf("GOARCH=%s", target.Arch),
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Build failed for %s/%s: %v\n", target.OS, target.Arch, err)
			os.Exit(1)
		}
	}

	// Copy documentation files
	fmt.Println("Copying documentation...")
	docFiles := []string{"README.md", "CHANGELOG.md", "LICENSE", "sample.vmx"}
	for _, file := range docFiles {
		if err := copyFile(file, filepath.Join(buildDir, file)); err != nil {
			fmt.Printf("Warning: Failed to copy %s: %v\n", file, err)
		}
	}

	fmt.Println("All builds complete!")
}

func createDist() {
	version, _, _ := getVersionInfo()

	// Ensure dist directory exists
	if err := os.MkdirAll(distDir, 0755); err != nil {
		fmt.Printf("Failed to create dist directory: %v\n", err)
		os.Exit(1)
	}

	zipFile := filepath.Join(distDir, fmt.Sprintf("vmxtool-%s.zip", version))
	shaFile := filepath.Join(distDir, fmt.Sprintf("vmxtool-%s.sha256", version))

	fmt.Printf("Creating distribution archive: %s\n", zipFile)

	// Create zip file
	if err := createZipArchive(buildDir, zipFile); err != nil {
		fmt.Printf("Failed to create zip: %v\n", err)
		os.Exit(1)
	}

	// Create SHA256 checksum
	if err := createSHA256(zipFile, shaFile); err != nil {
		fmt.Printf("Failed to create checksum: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Distribution created: %s\n", zipFile)
	fmt.Printf("Checksum: %s\n", shaFile)
}

func clean() {
	fmt.Println("Cleaning build artifacts...")

	// Remove build directory
	if err := os.RemoveAll(buildDir); err != nil {
		fmt.Printf("Warning: Failed to remove %s: %v\n", buildDir, err)
	}

	// Remove dist directory
	if err := os.RemoveAll(distDir); err != nil {
		fmt.Printf("Warning: Failed to remove %s: %v\n", distDir, err)
	}

	// Remove current platform binary
	if runtime.GOOS == "windows" {
		os.Remove("vmxtool.exe")
	} else {
		os.Remove("vmxtool")
	}

	fmt.Println("Clean complete!")
}

func runAll() {
	fmt.Println("=== Running all tasks ===")
	fmt.Println()

	fmt.Println("1. Running tests...")
	runTests()
	fmt.Println()

	fmt.Println("2. Building all platforms...")
	buildAll()
	fmt.Println()

	fmt.Println("3. Creating distribution...")
	createDist()
	fmt.Println()

	fmt.Println("=== All tasks complete! ===")
}

// Helper functions

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination directory if needed
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func createZipArchive(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Convert to forward slashes for zip (cross-platform compatibility)
		relPath = filepath.ToSlash(relPath)

		// Create zip entry
		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

func createSHA256(filePath, shaPath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	checksum := fmt.Sprintf("%x  %s\n", hash.Sum(nil), filepath.Base(filePath))

	return os.WriteFile(shaPath, []byte(checksum), 0644)
}
