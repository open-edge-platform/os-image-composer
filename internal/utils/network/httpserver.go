package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
)

// ServeRepositoryHTTP starts a temporary HTTP server to serve the repository
// Returns the server URL, cleanup function, and error
func ServeRepositoryHTTP(repoPath string) (serverURL string, cleanup func(), err error) {
	log := logger.Logger()

	// Validate repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return "", nil, fmt.Errorf("repository directory does not exist: %s", repoPath)
	} else if err != nil {
		return "", nil, fmt.Errorf("failed to access repository directory: %w", err)
	}

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", nil, fmt.Errorf("failed to find available port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Create HTTP server with file handler
	mux := http.NewServeMux()
	fileHandler := http.FileServer(http.Dir(repoPath))
	mux.Handle("/", fileHandler)

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: mux,
	}

	// Start server in goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		log.Infof("starting HTTP server for repository at http://localhost:%d", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
		close(serverErrChan)
	}()

	// Wait briefly to ensure server starts
	select {
	case err := <-serverErrChan:
		return "", nil, fmt.Errorf("failed to start HTTP server: %w", err)
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
	}

	serverURL = fmt.Sprintf("http://localhost:%d", port)
	log.Infof("repository available at: %s", serverURL)

	// Cleanup function
	cleanup = func() {
		log.Infof("shutting down HTTP server for repository")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Errorf("error shutting down HTTP server: %v", err)
		}
	}

	return serverURL, cleanup, nil
}
