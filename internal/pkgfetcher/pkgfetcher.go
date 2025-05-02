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
	"github.com/schollz/progressbar/v3"
	"go.uber.org/zap"
)

// FetchPackages downloads the given URLs into destDir using a pool of workers.
// It shows a single progress bar tracking files completed vs total.
func FetchPackages(urls []string, destDir string, workers int) error {
	logger := zap.L().Sugar()

	total := len(urls)
	jobs := make(chan string, total)
	var wg sync.WaitGroup

	// create a single progress bar for total files
	bar := progressbar.NewOptions(total,
		progressbar.OptionFullWidth(),
		progressbar.OptionShowDescriptionAtLineEnd(),
  		progressbar.OptionSetDescription("downloading"),
		  progressbar.OptionSetWidth(40),
  		progressbar.OptionShowCount(),
  		progressbar.OptionThrottle(100 * time.Millisecond),
  		
	)

	// start worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range jobs {
				name := path.Base(url)

				// update description to current file
				bar.Describe(fmt.Sprintf("downloading %s", name))

				err := func() error {
					resp, err := http.Get(url)
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("bad status: %s", resp.Status)
					}

					// ensure destination directory exists
					if err := os.MkdirAll(destDir, 0755); err != nil {
						return err
					}

					destPath := filepath.Join(destDir, name)
					out, err := os.Create(destPath)
					if err != nil {
						return err
					}
					defer out.Close()

					// write body to file
					if _, err := io.Copy(out, resp.Body); err != nil {
						return err
					}
					return nil
				}()

				if err != nil {
					logger.Errorf("downloading %s failed: %v", url, err)
				} 
				// increment progress bar
				bar.Add(1)
			}
		}()
	}

	// enqueue jobs
	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	wg.Wait()
	bar.Finish()
	return nil
}
