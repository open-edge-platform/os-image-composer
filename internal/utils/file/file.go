package file

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/utils/shell"
	"gopkg.in/yaml.v3"
)

// IsSubPath checks if the target path is a subpath of the base path
func IsSubPath(base, target string) (bool, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false, err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false, err
	}
	// rel == "." means same dir, rel starting with ".." means not subpath
	if rel == "." {
		return true, nil
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return false, nil
	}
	return true, nil
}

func getCurrentDirPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current directory path")
	}
	return filepath.Dir(filename), nil
}

// GetRootPath returns the root path of the application
func GetRootPath() (string, error) {
	currentPath, err := getCurrentDirPath()
	if err != nil {
		return "", err
	}
	utilsPath := filepath.Dir(currentPath)
	internalPath := filepath.Dir(utilsPath)
	rootPath := filepath.Dir(internalPath)
	return rootPath, nil
}

func GetGeneralConfigDir() (string, error) {
	rootPath, err := GetRootPath()
	if err != nil {
		return "", fmt.Errorf("failed to get root path: %w", err)
	}
	configDir := filepath.Join(rootPath, "config", "general")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return "", fmt.Errorf("general config directory does not exist: %s", configDir)
	}
	return configDir, nil
}

func GetTargetOsConfigDir(targetOs, targetDist string) (string, error) {
	rootPath, err := GetRootPath()
	if err != nil {
		return "", fmt.Errorf("failed to get root path: %w", err)
	}
	configDir := filepath.Join(rootPath, "config", "osv", targetOs, targetDist)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return "", fmt.Errorf("target OS config directory does not exist: %s", configDir)
	}
	return configDir, nil
}

// Append appends a string to the end of file dst.
func Append(data string, dst string) error {
	dstFile, err := os.OpenFile(dst, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s for appending: %w", dst, err)
	}
	defer dstFile.Close()

	_, err = dstFile.WriteString(data)
	return err
}

// ReadFromJSON reads a JSON file and returns its contents as a map
// If the file doesn't exist or is empty, returns an empty map
func ReadFromJSON(jsonFile string) (map[string]interface{}, error) {
	// Initialize empty map for result
	result := make(map[string]interface{})

	// Check if file exists
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		return result, fmt.Errorf("file does not exist: %s", jsonFile)
	}

	// Open the file
	file, err := os.Open(jsonFile)
	if err != nil {
		return result, err
	}
	defer file.Close()

	// Get file info to check if it's empty
	fileInfo, err := file.Stat()
	if err != nil {
		return result, err
	}

	// Return empty map if file is empty
	if fileInfo.Size() == 0 {
		return result, fmt.Errorf("file is empty: %s", jsonFile)
	}

	// Decode JSON content
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}

// WriteToJSON writes a map to a JSON file with specified indentation
func WriteToJSON(jsonFile string, data map[string]interface{}, indent int) error {
	// Create or truncate the file
	file, err := os.Create(jsonFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create encoder with indentation
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", strings.Repeat(" ", indent))

	// Encode and write the data
	if err := encoder.Encode(data); err != nil {
		return err
	}

	return nil
}

func decodeYAML(file *os.File, result *map[interface{}]interface{}) error {
	decoder := yaml.NewDecoder(file)
	var raw map[interface{}]interface{}
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	*result = raw
	return nil
}

func ReadFromYaml(yamlFile string) (map[interface{}]interface{}, error) {
	// Initialize empty map for result
	result := make(map[interface{}]interface{})

	// Check if file exists
	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		return result, fmt.Errorf("file does not exist: %s", yamlFile)
	}

	// Open the file
	file, err := os.Open(yamlFile)
	if err != nil {
		return result, err
	}
	defer file.Close()

	// Get file info to check if it's empty
	fileInfo, err := file.Stat()
	if err != nil {
		return result, err
	}

	// Return empty map if file is empty
	if fileInfo.Size() == 0 {
		return result, fmt.Errorf("file is empty: %s", yamlFile)
	}

	// Decode YAML content (assuming a function DecodeYAML exists)
	if err := decodeYAML(file, &result); err != nil {
		return result, err
	}

	return result, nil
}

func CopyFile(srcFile, dstFile, flags string, sudo bool) error {
	srcFilePath, err := filepath.Abs(srcFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of source file: %w", err)
	}
	if _, err := os.Stat(srcFilePath); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", srcFilePath)
	}

	dstFilePath, err := filepath.Abs(dstFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of destination file: %w", err)
	}
	dstDir := filepath.Dir(dstFilePath)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for destination file: %w", err)
		}
	}
	var cmdStr string
	if flags == "" {
		cmdStr = fmt.Sprintf("cp %s %s", srcFilePath, dstFilePath)
	} else {
		cmdStr = fmt.Sprintf("cp %s %s %s", flags, srcFilePath, dstFilePath)
	}
	if _, err := shell.ExecCmd(cmdStr, sudo, "", nil); err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcFilePath, dstFilePath, err)
	}
	return nil
}

func CopyDir(srcDir, dstDir, flags string, sudo bool) error {
	srcDirPath, err := filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of source directory: %w", err)
	}
	if _, err := os.Stat(srcDirPath); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", srcDirPath)
	}

	dstDirPath, err := filepath.Abs(dstDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of destination directory: %w", err)
	}
	if _, err := os.Stat(dstDirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dstDirPath, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	var cmdStr string
	if flags == "" {
		cmdStr = fmt.Sprintf("cp -r %s/* %s", srcDirPath, dstDirPath)
	} else {
		cmdStr = fmt.Sprintf("cp -r %s %s/* %s", flags, srcDirPath, dstDirPath)
	}
	if _, err := shell.ExecCmd(cmdStr, sudo, "", nil); err != nil {
		return fmt.Errorf("failed to copy directory from %s to %s: %w", srcDirPath, dstDirPath, err)
	}
	return nil
}
