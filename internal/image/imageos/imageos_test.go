package imageos

import (
	"bytes"
	"os"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
)

func TestBuildImageUKI_CaptureAllOutput(t *testing.T) {
	installRoot := t.TempDir()
	tmpl := &config.ImageTemplate{} // fill fields if needed

	// Capture all stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var output string

	defer func() {
		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output = buf.String()

		// Print captured output for debugging
		t.Logf("Captured output:\n%s", output)

		// Check that no panic occurred and function returned normally
		if rec := recover(); rec != nil {
			t.Errorf("unexpected panic: %v", rec)
		}
	}()

	err := buildImageUKI(installRoot, tmpl)
	if err != nil {
		t.Errorf("buildImageUKI returned error: %v", err)
	}
}
