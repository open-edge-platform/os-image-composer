package imageinspect

import (
	"fmt"
	"regexp"
	"strings"
)

// uuidRegex matches UUID format: 8-4-4-4-12 hex digits (case-insensitive with optional hyphens)
var uuidRegex = regexp.MustCompile(`[0-9a-fA-F]{8}[-_]?[0-9a-fA-F]{4}[-_]?[0-9a-fA-F]{4}[-_]?[0-9a-fA-F]{4}[-_]?[0-9a-fA-F]{12}`)

// extractUUIDsFromString finds all UUIDs in a string and returns them normalized.
func extractUUIDsFromString(s string) []string {
	if s == "" {
		return nil
	}
	matches := uuidRegex.FindAllString(s, -1)
	if matches == nil {
		return nil
	}
	// Deduplicate and normalize
	seen := make(map[string]struct{})
	var result []string
	for _, m := range matches {
		normalized := normalizeUUID(m)
		if _, ok := seen[normalized]; !ok {
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}
	return result
}

// normalizeUUID removes hyphens and converts to lowercase.
func normalizeUUID(uuid string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(uuid, "-", ""), "_", ""))
}

// parseGrubConfigContent extracts boot entries and kernel references from grub.cfg content.
func parseGrubConfigContent(content string) BootloaderConfig {
	cfg := BootloaderConfig{
		ConfigRaw:        make(map[string]string),
		KernelReferences: []KernelReference{},
		BootEntries:      []BootEntry{},
		UUIDReferences:   []UUIDReference{},
		Issues:           []string{},
	}

	if content == "" {
		cfg.Issues = append(cfg.Issues, "grub.cfg is empty")
		return cfg
	}

	// Store raw content (truncated if too large)
	if len(content) > 10240 { // 10KB limit
		cfg.ConfigRaw["grub.cfg"] = content[:10240] + "\n[truncated...]"
	} else {
		cfg.ConfigRaw["grub.cfg"] = content
	}

	// Extract UUIDs from config content
	uuids := extractUUIDsFromString(content)
	for _, uuid := range uuids {
		cfg.UUIDReferences = append(cfg.UUIDReferences, UUIDReference{
			UUID:    uuid,
			Context: "grub_config",
		})
	}

	// Extract critical metadata from the config
	lines := strings.Split(content, "\n")
	var configfilePath string
	var grubPrefix string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Extract set prefix value: set prefix=($root)"/boot/grub2"
		if strings.HasPrefix(trimmed, "set prefix") {
			parts := strings.Split(trimmed, "=")
			if len(parts) == 2 {
				prefixVal := strings.TrimSpace(parts[1])
				prefixVal = strings.Trim(prefixVal, `"'`)
				// If it contains ($root), we'll expand it when we find the root value
				if strings.HasPrefix(prefixVal, "(") && strings.Contains(prefixVal, ")") {
					// Extract the path part: ($root)"/boot/grub2" -> /boot/grub2
					if idx := strings.Index(prefixVal, ")"); idx >= 0 {
						grubPrefix = strings.Trim(prefixVal[idx+1:], `"'`)
					}
				} else {
					grubPrefix = prefixVal
				}
			}
		}

		// Look for configfile directive (loads external config)
		if strings.HasPrefix(trimmed, "configfile") {
			parts := strings.Fields(trimmed)
			if len(parts) > 1 {
				configfilePath = strings.Trim(parts[1], `"'`)
				// Remove variable prefix if present
				if strings.HasPrefix(configfilePath, "(") {
					// Format like ($root)"/boot/grub2/grub.cfg"
					if idx := strings.Index(configfilePath, ")"); idx >= 0 {
						configfilePath = configfilePath[idx+1:]
						configfilePath = strings.Trim(configfilePath, `"'`)
					}
				} else if strings.HasPrefix(configfilePath, "$prefix") {
					// Expand $prefix variable
					if grubPrefix != "" {
						configfilePath = strings.Replace(configfilePath, "$prefix", grubPrefix, 1)
					}
				}
			}
		}
	}

	// If this is a stub config (has configfile), add metadata note
	if configfilePath != "" {
		note := fmt.Sprintf("Configuration note: This is a UEFI stub config that loads the main GRUB configuration from the root partition at '%s'. The actual boot entries are defined in that file.", configfilePath)
		cfg.Issues = append(cfg.Issues, note)

		// Add a synthetic entry showing where the config is
		stubEntry := BootEntry{
			Name:   "[External config] " + configfilePath,
			Kernel: configfilePath,
		}
		cfg.BootEntries = append(cfg.BootEntries, stubEntry)
	}

	// Simple parsing of menuentry blocks (for cases where config is inline)
	var currentEntry *BootEntry
	var inMenuEntry bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect menuentry start
		if strings.HasPrefix(trimmed, "menuentry") {
			if currentEntry != nil {
				cfg.BootEntries = append(cfg.BootEntries, *currentEntry)
			}
			currentEntry = parseGrubMenuEntry(trimmed)
			inMenuEntry = true
			continue
		}

		if inMenuEntry && currentEntry != nil {
			// Parse commonbootloader options
			if strings.HasPrefix(trimmed, "linux") || strings.HasPrefix(trimmed, "vmlinuz") {
				parts := strings.Fields(trimmed)
				if len(parts) > 1 {
					currentEntry.Kernel = parts[1]
					if len(parts) > 2 {
						currentEntry.Cmdline = strings.Join(parts[2:], " ")
					}
				}
			}
			if strings.HasPrefix(trimmed, "initrd") {
				parts := strings.Fields(trimmed)
				if len(parts) > 1 {
					currentEntry.Initrd = parts[1]
				}
			}

			// Check for root device reference
			if strings.Contains(trimmed, "root=") {
				if idx := strings.Index(trimmed, "root="); idx >= 0 {
					rest := trimmed[idx+5:]
					// Extract the device/UUID value (up to next space)
					if spaceIdx := strings.IndexByte(rest, ' '); spaceIdx >= 0 {
						currentEntry.RootDevice = rest[:spaceIdx]
					} else {
						currentEntry.RootDevice = rest
					}
				}
			}

			// End of entry (closing brace or next menuentry)
			if strings.HasPrefix(trimmed, "}") {
				inMenuEntry = false
			}
		}
	}

	// Add last entry if exists
	if currentEntry != nil {
		cfg.BootEntries = append(cfg.BootEntries, *currentEntry)
	}

	// Extract kernel references
	for _, entry := range cfg.BootEntries {
		if entry.Kernel != "" {
			ref := KernelReference{
				Path:      entry.Kernel,
				BootEntry: entry.Name,
			}
			if entry.RootDevice != "" {
				ref.RootUUID = entry.RootDevice
			}
			cfg.KernelReferences = append(cfg.KernelReferences, ref)
		}
	}

	return cfg
}

// parseGrubMenuEntry extracts title/name from a menuentry line.
func parseGrubMenuEntry(menuLine string) *BootEntry {
	entry := &BootEntry{}

	// Extract text between quotes: menuentry 'Title' { or menuentry "Title" {
	for _, q := range []rune{'\'', '"'} {
		start := strings.IndexRune(menuLine, q)
		if start >= 0 {
			end := strings.IndexRune(menuLine[start+1:], q)
			if end >= 0 {
				entry.Name = menuLine[start+1 : start+1+end]
				return entry
			}
		}
	}

	// Fallback: extract whatever is between menuentry and {
	if idx := strings.Index(menuLine, "{"); idx > 0 {
		entry.Name = strings.TrimSpace(menuLine[9:idx])
	}

	return entry
}

// parseSystemdBootEntries extracts boot entries from systemd-boot loader config.
func parseSystemdBootEntries(content string) BootloaderConfig {
	cfg := BootloaderConfig{
		ConfigRaw:        make(map[string]string),
		KernelReferences: []KernelReference{},
		BootEntries:      []BootEntry{},
		UUIDReferences:   []UUIDReference{},
		Issues:           []string{},
	}

	if content == "" {
		cfg.Issues = append(cfg.Issues, "loader.conf is empty")
		return cfg
	}

	if len(content) > 10240 {
		cfg.ConfigRaw["loader.conf"] = content[:10240] + "\n[truncated...]"
	} else {
		cfg.ConfigRaw["loader.conf"] = content
	}

	// Extract UUIDs from config
	uuids := extractUUIDsFromString(content)
	for _, uuid := range uuids {
		cfg.UUIDReferences = append(cfg.UUIDReferences, UUIDReference{
			UUID:    uuid,
			Context: "systemd_boot_config",
		})
	}

	// Parse simple key=value pairs
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "default") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				cfg.DefaultEntry = strings.TrimSpace(parts[1])
			}
		}
	}

	return cfg
}

// parseEFIBootEntries extracts boot entries from EFI boot variables (efivars style).
func parseEFIBootEntries(entryContent string) []BootEntry {
	entries := []BootEntry{}
	// This is a simplified parser; real EFI boot entry parsing is complex
	// For now, just extract any kernel paths and UUIDs mentioned
	lines := strings.Split(entryContent, "\n")
	for _, line := range lines {
		if strings.Contains(line, "File") || strings.Contains(line, "kernel") {
			if path := extractPathFromEFIVariable(line); path != "" {
				entries = append(entries, BootEntry{
					Kernel: path,
					Name:   "EFI_Entry",
				})
			}
		}
	}
	return entries
}

// extractPathFromEFIVariable attempts to extract a file path from EFI variable output.
func extractPathFromEFIVariable(line string) string {
	// Look for common path patterns
	patterns := []string{
		`File\(.+?\)`,
		`\\[A-Z0-9\s\\]+\.efi`,
		`/[A-Za-z0-9/_.-]+`,
	}

	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		matches := re.FindAllString(line, 1)
		if len(matches) > 0 {
			return strings.TrimSpace(matches[0])
		}
	}

	return ""
}

// resolveUUIDsToPartitions matches UUIDs in bootloader config against partition GUIDs.
// It returns a map of UUID -> partition index.
func resolveUUIDsToPartitions(uuidRefs []UUIDReference, pt PartitionTableSummary) map[string]int {
	result := make(map[string]int)

	for _, ref := range uuidRefs {
		normalized := normalizeUUID(ref.UUID)
		for _, p := range pt.Partitions {
			if normalizeUUID(p.GUID) == normalized {
				result[ref.UUID] = p.Index
				break
			}
			// Also check filesystem UUID
			if p.Filesystem != nil && normalizeUUID(p.Filesystem.UUID) == normalized {
				result[ref.UUID] = p.Index
				break
			}
		}
	}

	return result
}

// ValidateBootloaderConfig checks for common configuration issues.
func ValidateBootloaderConfig(cfg *BootloaderConfig, pt PartitionTableSummary) {
	if cfg == nil {
		return
	}

	// Check for missing config files
	if len(cfg.ConfigFiles) == 0 && len(cfg.ConfigRaw) == 0 {
		cfg.Issues = append(cfg.Issues, "No bootloader configuration files found")
	}

	// Resolve UUIDs and check for mismatches
	uuidMap := resolveUUIDsToPartitions(cfg.UUIDReferences, pt)
	for i, uuidRef := range cfg.UUIDReferences {
		if _, found := uuidMap[uuidRef.UUID]; found {
			cfg.UUIDReferences[i].ReferencedPartition = uuidMap[uuidRef.UUID]
		} else {
			cfg.UUIDReferences[i].Mismatch = true
			cfg.Issues = append(cfg.Issues,
				fmt.Sprintf("UUID %s referenced in %s not found in partition table", uuidRef.UUID, uuidRef.Context))
		}
	}

	// Check for kernel references without valid paths
	for _, kernRef := range cfg.KernelReferences {
		if kernRef.Path == "" {
			cfg.Issues = append(cfg.Issues, fmt.Sprintf("Boot entry %s has no kernel path", kernRef.BootEntry))
		}
	}

	// Check for boot entries without kernel
	for _, entry := range cfg.BootEntries {
		if entry.Kernel == "" {
			cfg.Issues = append(cfg.Issues, fmt.Sprintf("Boot entry '%s' has no kernel path", entry.Name))
		}
	}
}
