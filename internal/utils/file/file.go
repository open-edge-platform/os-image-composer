package file

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
