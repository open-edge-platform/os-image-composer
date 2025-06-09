package slice

// ConvertToStringSlice converts an interface{} slice to a string slice
func ConvertToStringSlice(input interface{}) ([]string, bool) {
	slice, ok := input.([]interface{})
	if !ok {
		return nil, false
	}

	result := make([]string, len(slice))
	for i, v := range slice {
		result[i], ok = v.(string)
		if !ok {
			return nil, false
		}
	}

	return result, true
}

// ConvertToInterfaceSlice converts a string slice to an interface{} slice
func ConvertToInterfaceSlice(input []string) []interface{} {
	result := make([]interface{}, len(input))
	for i, v := range input {
		result[i] = v
	}
	return result
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
