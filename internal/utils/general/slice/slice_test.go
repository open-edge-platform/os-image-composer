package slice_test

import (
	"testing"

	"github.com/open-edge-platform/image-composer/internal/utils/general/slice"
)

func TestConvertToStringSlice(t *testing.T) {
	input := []interface{}{"a", "b", "c"}
	expected := []string{"a", "b", "c"}
	result, ok := slice.ConvertToStringSlice(input)
	if !ok {
		t.Fatalf("ConvertToStringSlice failed to convert valid input")
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("Expected %s, got %s", v, result[i])
		}
	}

	invalidInput := []interface{}{"a", 2, "c"}
	_, ok = slice.ConvertToStringSlice(invalidInput)
	if ok {
		t.Errorf("ConvertToStringSlice should fail for non-string elements")
	}
}

func TestConvertToInterfaceSlice(t *testing.T) {
	input := []string{"x", "y"}
	result := slice.ConvertToInterfaceSlice(input)
	if len(result) != len(input) {
		t.Fatalf("Expected length %d, got %d", len(input), len(result))
	}
	for i, v := range input {
		if result[i] != v {
			t.Errorf("Expected %v, got %v", v, result[i])
		}
	}
}

func TestContains(t *testing.T) {
	_slice := []string{"foo", "bar"}
	if !slice.Contains(_slice, "foo") {
		t.Errorf("Contains should return true for existing element")
	}
	if slice.Contains(_slice, "baz") {
		t.Errorf("Contains should return false for non-existing element")
	}
}

func TestContainsInterface(t *testing.T) {
	_slice := []interface{}{"apple", "banana"}
	if !slice.ContainsInterface(_slice, "banana") {
		t.Errorf("ContainsInterface should return true for existing element")
	}
	if slice.ContainsInterface(_slice, "orange") {
		t.Errorf("ContainsInterface should return false for non-existing element")
	}
}

func TestContainsInterfaceMapKey(t *testing.T) {
	m := map[string]interface{}{"key1": 1, "key2": 2}
	if !slice.ContainsInterfaceMapKey(m, "key1") {
		t.Errorf("ContainsInterfaceMapKey should return true for existing key")
	}
	if slice.ContainsInterfaceMapKey(m, "key3") {
		t.Errorf("ContainsInterfaceMapKey should return false for non-existing key")
	}
}

func TestContainsStringMapKey(t *testing.T) {
	m := map[string]string{"a": "A", "b": "B"}
	if !slice.ContainsStringMapKey(m, "a") {
		t.Errorf("ContainsStringMapKey should return true for existing key")
	}
	if slice.ContainsStringMapKey(m, "c") {
		t.Errorf("ContainsStringMapKey should return false for non-existing key")
	}
}
