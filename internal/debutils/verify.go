package debutils

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"crypto/sha256"
	"io"
	"os"

	"github.com/schollz/progressbar/v3"
	"go.uber.org/zap"
)

// Result holds the outcome of verifying one RPM.
type Result struct {
	Path     string        // filesystem path to the .rpm
	OK       bool          // signature + checksum OK?
	Duration time.Duration // how long the check took
	Error    error         // any error (signature fail, I/O, etc)
}

// VerifyAll takes a slice of RPM file paths, verifies each one in parallel,
// and returns a slice of results in the same order.
func VerifyAll(paths []string, pkgChecksum map[string]string, pubkeyPath string, workers int) []Result {
	logger := zap.L().Sugar()

	logger.Infof("Verifying %d packages with %d workers", len(paths), workers)

	total := len(paths)
	results := make([]Result, total) // allocate up front
	jobs := make(chan int, total)    // channel of indices
	var wg sync.WaitGroup

	// build the progress bar
	bar := progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowDescriptionAtLineEnd(),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(30),
		progressbar.OptionThrottle(200*time.Millisecond),
		progressbar.OptionSpinnerType(10),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()
			for idx := range jobs {
				debPath := paths[idx]
				name := filepath.Base(debPath)
				bar.Describe("verifying " + name)

				start := time.Now()
				err := verifyWithGoDeb(debPath, pkgChecksum)
				ok := err == nil

				if err != nil {
					logger.Errorf("verification %s failed: %v", debPath, err)
				}

				results[idx] = Result{
					Path:     debPath,
					OK:       ok,
					Duration: time.Since(start),
					Error:    err,
				}

				if err := bar.Add(1); err != nil {
					logger.Errorf("failed to add to progress bar: %v", err)
				}
			}
		}(i)
	}

	// enqueue indices
	for i := range paths {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
	if err := bar.Finish(); err != nil {
		logger.Errorf("failed to finish progress bar: %v", err)
	}

	return results
}

// checksumWithGoDeb verifies the checksum of a .deb file using the GoDeb library.
func verifyWithGoDeb(deb string, pkgChecksum map[string]string) error {

	checksum := getChecksumByName(pkgChecksum, deb)
	// fmt.Printf("File: %s, Checksum: %s\n", deb, checksum)

	// Here you would implement the actual checksum verification logic
	if checksum == "NOT FOUND" {
		return fmt.Errorf("no checksum found for %s", deb)
	}

	actual, err := computeFileSHA256(deb)
	if err != nil {
		return fmt.Errorf("failed to compute checksum for %s: %w", deb, err)
	}

	if !strings.EqualFold(actual, checksum) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", deb, checksum, actual)
	}

	return nil
}

func getChecksumByName(pkgChecksum map[string]string, deb string) string {

	// Extract the base file name without directory and version
	// Example: "apt-config-icons-large-hidpi_0.16.1-2_all.deb" -> "apt-config-icons-large-hidpi"
	base := filepath.Base(deb)
	name := base
	if idx := strings.Index(base, "_"); idx != -1 {
		name = base[:idx]
	}

	for k, v := range pkgChecksum {
		if name == k {
			return v
		}
	}
	return "NOT FOUND"
}

// computeFileSHA256 computes the SHA256 checksum of the given file.
func computeFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
