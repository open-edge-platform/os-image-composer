package convert

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Constants for size conversions
const (
	KiB = 1024                 // 1 KiB = 1024 bytes
	Kb  = 1000                 // 1 Kb = 1000 bytes
	MiB = (1024 * 1024)        // 1 MiB = 1024 KiB * 1024 bytes
	MB  = (1000 * 1000)        // 1 MB = 1000 KiB * 1000 bytes
	GiB = (1024 * 1024 * 1024) // 1 GiB = 1024 MiB
	GB  = (1000 * 1000 * 1000) // 1 GB = 1000 MB * 1000 bytes
)

// sizeMap maps unit strings to their byte multipliers.
// Using a map makes the logic clean and easily extensible.
var sizeMap = map[string]uint64{
	"KIB": KiB,
	"KB":  Kb,
	"MIB": MiB,
	"MB":  MB,
	"GIB": GiB,
	"GB":  GB,
}

// SizeRegex is a compiled regular expression to efficiently parse size strings.
var sizeRegex = regexp.MustCompile(`^(\d+)\s*([A-Z]+)$`)

// NormalizeSizeToBytes converts a size string (e.g., "513MiB") to bytes.
func NormalizeSizeToBytes(sizeStr string) (uint64, error) {
	// Convert the input string to uppercase to make matching case-insensitive.
	// The regex is also case-insensitive for the unit part.
	upperSizeStr := strings.ToUpper(sizeStr)

	matches := sizeRegex.FindStringSubmatch(upperSizeStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid size format: %q", sizeStr)
	}

	// matches[1] is the numeric part, matches[2] is the unit part.
	numericPart := matches[1]
	unitPart := matches[2]

	// Parse the numeric string into an unsigned 64-bit integer.
	value, err := strconv.ParseUint(numericPart, 10, 64)
	if err != nil {
		// This can happen if the number is too large to fit in a uint64.
		return 0, fmt.Errorf("failed to parse numeric value %q: %w", numericPart, err)
	}

	// Look up the multiplier from the map.
	multiplier, ok := sizeMap[unitPart]
	if !ok {
		return 0, fmt.Errorf("unknown size unit: %q", matches[2])
	}

	// Calculate the final size in bytes.
	return value * multiplier, nil
}
