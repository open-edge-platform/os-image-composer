package imageconvert_test

import (
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imageconvert"
)

func TestConvertImageFile_ValidParameters(t *testing.T) {
	filePath := "/tmp/test-image.raw"
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
	}

	imageConvert := imageconvert.NewImageConvert()
	err := imageConvert.ConvertImageFile(filePath, template)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}
