package logger

import (
	"fmt"
	"os"
	"path/filepath"
)

type StringListReport struct {
	Title string
	Items []string
}

var GlobalStringListReport StringListReport
var ReportPath = "builds"

func init() {
	GlobalStringListReport = StringListReport{
		Title: "FetchedFiles",
		Items: []string{},
	}
}

// WriteListFetchedToFile writes the GlobalStringListReport to a text file as a list.
// The title is appended to the filename, e.g., fetchurl-title.txt.
func WriteListFetchedToFile() error {
	if err := os.MkdirAll(ReportPath, 0755); err != nil {
		return fmt.Errorf("creating base path: %w", err)
	}

	// Sanitize the title for use in a filename
	title := GlobalStringListReport.Title
	if title == "" {
		title = "untitled"
	}
	// Replace spaces and special characters with underscores
	safeTitle := ""
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			safeTitle += string(r)
		} else {
			safeTitle += "_"
		}
	}

	reportFullPath := filepath.Join(ReportPath, fmt.Sprintf("fetchurl-%s.txt", safeTitle))

	f, err := os.OpenFile(reportFullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// Write each item in the list
	for _, item := range GlobalStringListReport.Items {
		if _, err := fmt.Fprintln(f, item); err != nil {
			return fmt.Errorf("writing to file: %w", err)
		}
	}

	GlobalStringListReport.Items = []string{}
	if _, err := fmt.Fprintln(f); err != nil {
		return fmt.Errorf("writing new line to file: %w", err)
	}

	return nil
}
