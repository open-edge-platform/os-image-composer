package file_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/utils/file"
)

func TestReadFromJSON_FileNotExist(t *testing.T) {
	_, err := file.ReadFromJSON("not_exist.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadFromJSON_EmptyFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "empty.json")
	err := os.WriteFile(tmp, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	_, err = file.ReadFromJSON(tmp)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestReadFromJSON_Valid(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.json")
	content := `{"foo": "bar", "num": 42}`
	err := os.WriteFile(tmp, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	m, err := file.ReadFromJSON(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["foo"] != "bar" || m["num"] != float64(42) {
		t.Errorf("unexpected map: %v", m)
	}
}

func TestWriteToJSON_And_ReadBack(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "out.json")
	data := map[string]interface{}{"a": 1, "b": "x"}
	err := file.WriteToJSON(tmp, data, 2)
	if err != nil {
		t.Fatalf("WriteToJSON failed: %v", err)
	}
	read, err := file.ReadFromJSON(tmp)
	if err != nil {
		t.Fatalf("ReadFromJSON failed: %v", err)
	}
	if !reflect.DeepEqual(read, map[string]interface{}{"a": float64(1), "b": "x"}) {
		t.Errorf("unexpected read: %v", read)
	}
}

func TestReadFromYaml_FileNotExist(t *testing.T) {
	_, err := file.ReadFromYaml("not_exist.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadFromYaml_EmptyFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "empty.yaml")
	err := os.WriteFile(tmp, []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	_, err = file.ReadFromYaml(tmp)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestReadFromYaml_Valid(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.yaml")
	content := "foo: bar\nnum: 42\n"
	err := os.WriteFile(tmp, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	m, err := file.ReadFromYaml(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["foo"] != "bar" || m["num"] != 42 {
		t.Errorf("unexpected map: %v", m)
	}
}
