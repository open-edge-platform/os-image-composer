package slice_test

import (
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/utils/slice"
)

func TestConvertToSliceOfT_String(t *testing.T) {
	input := []interface{}{"a", "b", "c"}
	expected := []string{"a", "b", "c"}
	result, ok := slice.ConvertToSliceOfT[string](input)
	if !ok {
		t.Fatalf("ConvertToSliceOfT[string] failed to convert valid input")
	}
	if len(result) != len(expected) {
		t.Fatalf("Expected length %d, got %d", len(expected), len(result))
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("Expected %s, got %s", v, result[i])
		}
	}

	invalidInput := []interface{}{"a", 2, "c"}
	_, ok = slice.ConvertToSliceOfT[string](invalidInput)
	if ok {
		t.Errorf("ConvertToSliceOfT[string] should fail for non-string elements")
	}
}

func TestConvertToSliceOfT_Int(t *testing.T) {
	input := []interface{}{1, 2, 3}
	expected := []int{1, 2, 3}
	result, ok := slice.ConvertToSliceOfT[int](input)
	if !ok {
		t.Fatalf("ConvertToSliceOfT[int] failed to convert valid input")
	}
	if len(result) != len(expected) {
		t.Fatalf("Expected length %d, got %d", len(expected), len(result))
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("Expected %d, got %d", v, result[i])
		}
	}

	invalidInput := []interface{}{1, "b", 3}
	_, ok = slice.ConvertToSliceOfT[int](invalidInput)
	if ok {
		t.Errorf("ConvertToSliceOfT[int] should fail for non-int elements")
	}
}

func TestConvertToSliceOfT_Empty(t *testing.T) {
	input := []interface{}{}
	result, ok := slice.ConvertToSliceOfT[string](input)
	if !ok {
		t.Fatalf("ConvertToSliceOfT should succeed for empty input")
	}
	if len(result) != 0 {
		t.Errorf("Expected empty result, got length %d", len(result))
	}
}

type customStruct struct {
	A int
	B string
}

func TestConvertToSliceOfT_CustomStruct(t *testing.T) {
	s1 := customStruct{A: 1, B: "x"}
	s2 := customStruct{A: 2, B: "y"}
	input := []interface{}{s1, s2}
	expected := []customStruct{s1, s2}
	result, ok := slice.ConvertToSliceOfT[customStruct](input)
	if !ok {
		t.Fatalf("ConvertToSliceOfT[customStruct] failed to convert valid input")
	}
	if len(result) != len(expected) {
		t.Fatalf("Expected length %d, got %d", len(expected), len(result))
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("Expected %+v, got %+v", v, result[i])
		}
	}

	invalidInput := []interface{}{s1, "not a struct"}
	_, ok = slice.ConvertToSliceOfT[customStruct](invalidInput)
	if ok {
		t.Errorf("ConvertToSliceOfT[customStruct] should fail for non-matching elements")
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
