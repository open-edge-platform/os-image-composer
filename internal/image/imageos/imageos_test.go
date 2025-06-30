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

		// Check for panic
		if rec := recover(); rec == nil {
			t.Errorf("")
		} else if rec != "hard stop: UKI configuration is not implemented" {
			t.Errorf("unexpected panic message: got %v, want %v", rec, "hard stop: UKI configuration is not implemented")
		}
	}()

	_ = buildImageUKI(installRoot, tmpl)
}
