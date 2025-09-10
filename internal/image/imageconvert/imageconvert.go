package imageconvert

import "github.com/open-edge-platform/image-composer/internal/config"

type ImageConvertInterface interface {
	ConvertImageFile(filePath string, template *config.ImageTemplate) error
}

type ImageConvert struct{}

func NewImageConvert() *ImageConvert {
	return &ImageConvert{}
}

func (imageConvert *ImageConvert) ConvertImageFile(filePath string, template *config.ImageTemplate) error {
	return nil
}
