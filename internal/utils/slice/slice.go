package slice

import "strings"

// ConvertToSliceOfT converts a slice of interface{} to a slice of type T
func ConvertToSliceOfT[T any](input []interface{}) ([]T, bool) {
	result := make([]T, len(input))
	for i, v := range input {
		val, ok := v.(T)
		if !ok {
			return nil, false
		}
		result[i] = val
	}
	return result, true
}

// Check if a string exists in a string slice
func Contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func ContainsSubstring(slice []string, substr string) string {
	for _, item := range slice {
		if strings.Contains(item, substr) {
			return item
		}
	}
	return ""
}

func ContainsPrefix(slice []string, prefix string) string {
	for _, item := range slice {
		if strings.HasPrefix(item, prefix) {
			return item
		}
	}
	return ""
}

// containsInterface checks if a string is present in a slice of interface{}
func ContainsInterface(slice []interface{}, item string) bool {
	for _, s := range slice {
		if s.(string) == item {
			return true
		}
	}
	return false
}

func ContainsInterfaceMapKey(m map[string]interface{}, key string) bool {
	_, ok := m[key]
	return ok
}

func ContainsStringMapKey(m map[string]string, key string) bool {
	_, ok := m[key]
	return ok
}

func RemoveStringFromSlice(slice []string, str string) []string {
	for i, item := range slice {
		if item == str {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// split string and clean
func SplitBySpace(s string) []string {
	parts := strings.Split(s, " ")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
