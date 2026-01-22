package imageinspect

import (
	"bytes"
	"debug/pe"
	"fmt"
	"sort"
	"strings"
)

// ParsePEFromBytes parses a PE (Portable Executable) binary from the given byte slice
func ParsePEFromBytes(p string, blob []byte) (EFIBinaryEvidence, error) {
	ev := EFIBinaryEvidence{
		Path:            p,
		Size:            int64(len(blob)),
		SectionSHA256:   map[string]string{},
		OSReleaseSorted: []KeyValue{},
		Kind:            classifyBootloaderKind(p, nil), // refine after we parse sections
	}

	// whole-file hash
	ev.SHA256 = sha256Hex(blob)

	r := bytes.NewReader(blob)
	f, err := pe.NewFile(r)
	if err != nil {
		return ev, err
	}
	defer f.Close()

	ev.Arch = peMachineToArch(f.FileHeader.Machine)

	// Sections
	for _, s := range f.Sections {
		name := strings.TrimRight(s.Name, "\x00")
		ev.Sections = append(ev.Sections, name)
	}

	// Signed evidence: presence of Authenticode blob
	signed, sigSize, sigNote := peSignatureInfo(f)
	ev.Signed = signed
	ev.SignatureSize = sigSize
	if sigNote != "" {
		ev.Notes = append(ev.Notes, sigNote)
	}

	// SBAT section presence
	ev.HasSBAT = hasSection(ev.Sections, ".sbat")

	// UKI detection: these sections are highly indicative
	isUKI := hasSection(ev.Sections, ".linux") &&
		(hasSection(ev.Sections, ".cmdline") || hasSection(ev.Sections, ".osrel") || hasSection(ev.Sections, ".uname"))
	ev.IsUKI = isUKI
	if isUKI {
		ev.Kind = BootloaderUKI
	} else {
		// reclassify based on name/path/sections
		ev.Kind = classifyBootloaderKind(p, ev.Sections)
	}

	// Hash & extract interesting sections
	// Note: s.Data() reads section contents from underlying ReaderAt.
	// For large payloads (.linux, .initrd), this is still OK because blob is already in memory.
	for _, s := range f.Sections {
		name := strings.TrimRight(s.Name, "\x00")
		data, err := s.Data()
		if err != nil {
			ev.Notes = append(ev.Notes, fmt.Sprintf("read section %s: %v", name, err))
			continue
		}
		ev.SectionSHA256[name] = sha256Hex(data)

		switch name {
		case ".linux":
			ev.KernelSHA256 = ev.SectionSHA256[name]
		case ".initrd":
			ev.InitrdSHA256 = ev.SectionSHA256[name]
		case ".cmdline":
			ev.CmdlineSHA256 = ev.SectionSHA256[name]
			ev.Cmdline = strings.TrimSpace(string(bytes.Trim(data, "\x00")))
		case ".uname":
			ev.UnameSHA256 = ev.SectionSHA256[name]
			ev.Uname = strings.TrimSpace(string(bytes.Trim(data, "\x00")))
		case ".osrel":
			ev.OSRelSHA256 = ev.SectionSHA256[name]
			raw := strings.TrimSpace(string(bytes.Trim(data, "\x00")))
			ev.OSReleaseRaw = raw
			ev.OSRelease, ev.OSReleaseSorted = parseOSRelease(raw)
		}
	}

	return ev, nil
}

// peSignatureInfo checks for the presence of an Authenticode signature in the PE file
func peSignatureInfo(f *pe.File) (signed bool, sigSize int, note string) {
	// IMAGE_DIRECTORY_ENTRY_SECURITY = 4
	const secDir = 4

	// OptionalHeader can be OptionalHeader32 or OptionalHeader64.
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		if len(oh.DataDirectory) > secDir {
			sz := oh.DataDirectory[secDir].Size
			va := oh.DataDirectory[secDir].VirtualAddress // file offset for security dir
			if sz > 0 && va > 0 {
				return true, int(sz), ""
			}
		}
	case *pe.OptionalHeader64:
		if len(oh.DataDirectory) > secDir {
			sz := oh.DataDirectory[secDir].Size
			va := oh.DataDirectory[secDir].VirtualAddress
			if sz > 0 && va > 0 {
				return true, int(sz), ""
			}
		}
	default:
		return false, 0, "unknown optional header type"
	}
	return false, 0, ""
}

// classifyBootloaderKind classifies the bootloader kind based on path and sections
func classifyBootloaderKind(p string, sections []string) BootloaderKind {
	lp := strings.ToLower(p)

	// Most deterministic first:
	if sections != nil && hasSection(sections, ".linux") {
		// likely UKI; caller can override with stricter check
		return BootloaderUKI
	}

	// Path / filename heuristics:
	// shim often includes "shim" and/or has .sbat too
	if strings.Contains(lp, "shim") {
		return BootloaderShim
	}
	if strings.Contains(lp, "systemd") && strings.Contains(lp, "boot") {
		return BootloaderSystemdBoot
	}
	if strings.Contains(lp, "grub") {
		return BootloaderGrub
	}
	if strings.Contains(lp, "mmx64.efi") || strings.Contains(lp, "mmia32.efi") {
		return BootloaderMokManager
	}
	// fallback
	return BootloaderUnknown
}

// hasSection checks if the given section name is present in the list (case-insensitive)
func hasSection(secs []string, want string) bool {
	want = strings.ToLower(want)
	for _, s := range secs {
		if strings.ToLower(strings.TrimSpace(s)) == want {
			return true
		}
	}
	return false
}

// peMachineToArch maps PE machine types to architecture strings
func peMachineToArch(m uint16) string {
	switch m {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return "x86_64"
	case pe.IMAGE_FILE_MACHINE_I386:
		return "x86"
	case pe.IMAGE_FILE_MACHINE_ARM64:
		return "arm64"
	case pe.IMAGE_FILE_MACHINE_ARM:
		return "arm"
	default:
		return fmt.Sprintf("unknown(0x%x)", m)
	}
}

// parseOSRelease parses os-release style key=value data.
func parseOSRelease(raw string) (map[string]string, []KeyValue) {
	m := map[string]string{}

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)

		// os-release allows quoted values.
		v = strings.Trim(v, `"'`)

		if k != "" {
			m[k] = v
		}
	}

	// deterministic ordering
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sorted := make([]KeyValue, 0, len(keys))
	for _, k := range keys {
		sorted = append(sorted, KeyValue{
			Key:   k,
			Value: m[k],
		})
	}

	return m, sorted
}
