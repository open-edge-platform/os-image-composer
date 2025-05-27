package rpmutils

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"os"

	"github.com/cavaliergopher/rpm"
	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/schollz/progressbar/v3"
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
func VerifyAll(paths []string, pubkeyPath string, workers int) []Result {
	logger := utils.Logger()

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
		go func() {
			defer wg.Done()
			for idx := range jobs {
				rpmPath := paths[idx]
				name := filepath.Base(rpmPath)
				bar.Describe("verifying " + name)

				start := time.Now()
				err := verifyWithGoRpm(rpmPath, pubkeyPath)
				ok := err == nil

				if err != nil {
					logger.Errorf("verification %s failed: %v", rpmPath, err)
				}

				results[idx] = Result{
					Path:     rpmPath,
					OK:       ok,
					Duration: time.Since(start),
					Error:    err,
				}

				if err := bar.Add(1); err != nil {
					logger.Errorf("failed to add to progress bar: %v", err)
				}
			}
		}()
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

// verifyWithGoRpm uses go-rpm to GPG-check + MD5-check a single file.
func verifyWithGoRpm(rpmPath, pubkeyPath string) error {

	// load the keyring
	keyring, err := rpm.OpenKeyRing(pubkeyPath)
	if err != nil {
		return fmt.Errorf("loading keyring: %w", err)
	}

	// open the RPM
	f, err := os.Open(rpmPath)
	if err != nil {
		return fmt.Errorf("opening rpm: %w", err)
	}
	defer f.Close()

	// GPG signature check
	if _, err := rpm.GPGCheck(f, keyring); err != nil {
		return fmt.Errorf("GPG check failed: %w", err)
	}
	// rewind for the next check
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("seek rpm: %w", err)
	}

	// MD5 digest check
	if err := rpm.MD5Check(f); err != nil {
		return fmt.Errorf("MD5 checksum failed: %w", err)
	}

	return nil
}
