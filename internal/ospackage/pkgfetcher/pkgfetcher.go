package pkgfetcher

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/network"
	"github.com/schollz/progressbar/v3"
)

// FetchPackages downloads the given URLs into destDir using a pool of workers.
// It shows a single progress bar tracking files completed vs total.
func FetchPackages(urls []string, destDir string, workers int) error {
	log := logger.Logger()

	total := len(urls)
	jobs := make(chan string, total)
	var wg sync.WaitGroup

	// create a single progress bar for total files
	bar := progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowDescriptionAtLineEnd(),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowCount(),
		progressbar.OptionThrottle(200*time.Millisecond),
		progressbar.OptionSpinnerType(10),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// create a shared boolean flag to signal a download error
	downloadError := false

	// start worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range jobs {
				name := path.Base(url)

				// update description to current file
				bar.Describe(name)

				// ensure destination directory exists
				if err := os.MkdirAll(destDir, 0755); err != nil {
					log.Errorf("failed to create dest dir %s: %v", destDir, err)
					if err := bar.Add(1); err != nil {
						log.Errorf("failed to add to progress bar: %v", err)
					}
					continue
				}

				destPath := filepath.Join(destDir, name)
				if fi, err := os.Stat(destPath); err == nil {
					if fi.Size() > 0 {
						//log.Debugf("skipping existing %s", name)
						if err := bar.Add(1); err != nil {
							log.Errorf("failed to add to progress bar: %v", err)
						}
						continue
					}
					// file exists but zero size: re-download
					log.Warnf("re-downloading zero-size %s", name)
				}
				err := func() error {

					client := network.NewSecureHTTPClient()
					resp, err := client.Get(url)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("bad status: %s", resp.Status)
					}

					out, err := os.Create(destPath)
					if err != nil {
						return err
					}
					defer out.Close()

					if _, err := io.Copy(out, resp.Body); err != nil {
						return err
					}
					return nil
				}()

				if err != nil {
					log.Errorf("downloading %s failed: %v", url, err)
					downloadError = true
				}
				// increment progress bar
				if err := bar.Add(1); err != nil {
					log.Errorf("failed to add to progress bar: %v", err)
				}
			}
		}()
	}

	// enqueue jobs
	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	wg.Wait()

	// error after all jobs done
	if downloadError {
		return fmt.Errorf("one or more downloads failed")
	}

	if err := bar.Finish(); err != nil {
		log.Errorf("failed to finish progress bar: %v", err)
	}
	return nil
}
