package debutils

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

func Decompress(inFile string, outFile string) ([]string, error) {

	gzFile, err := os.Open(inFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open gz file: %v", err)
	}
	defer gzFile.Close()

	decompressedFile := outFile
	outDecompressed, err := os.Create(decompressedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create decompressed file: %v", err)
	}
	defer outDecompressed.Close()

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	_, err = io.Copy(outDecompressed, gzReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress file: %v", err)
	}

	return []string{decompressedFile}, nil
}
