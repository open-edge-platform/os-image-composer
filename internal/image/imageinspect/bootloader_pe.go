package imageinspect

import (
	"bytes"
	"debug/pe"
	"fmt"
	"sort"
	"strings"
)

func ParsePEFromBytes(p string, blob []byte) (EFIBinaryEvidence, error) {
	ev := EFIBinaryEvidence{
		Path:            p,
		Size:            int64(len(blob)),
		SectionSHA256:   map[string]string{},
		OSReleaseSorted: []KeyValue{},
		Kind:            BootloaderUnknown, // set after we have more evidence
	}

	ev.SHA256 = sha256Hex(blob)

	r := bytes.NewReader(blob)
	f, err := pe.NewFile(r)
	if err != nil {
		return ev, err
	}
	defer f.Close()

	ev.Arch = peMachineToArch(f.FileHeader.Machine)

	for _, s := range f.Sections {
		name := strings.TrimRight(s.Name, "\x00")
		ev.Sections = append(ev.Sections, name)
	}

	signed, sigSize, sigNote := peSignatureInfo(f)
	ev.Signed = signed
	ev.SignatureSize = sigSize
	if sigNote != "" {
		ev.Notes = append(ev.Notes, sigNote)
	}

	ev.HasSBAT = hasSection(ev.Sections, ".sbat")

	isUKI := hasSection(ev.Sections, ".linux") &&
		(hasSection(ev.Sections, ".cmdline") || hasSection(ev.Sections, ".osrel") || hasSection(ev.Sections, ".uname"))
	ev.IsUKI = isUKI
	if isUKI {
		ev.Kind = BootloaderUKI
	} else {
		// First pass: path/sections only
		ev.Kind = classifyBootloaderKind(p, ev.Sections, nil, ev.HasSBAT)

		// If still unknown, do lightweight strings pass
		if ev.Kind == BootloaderUnknown {
			ev.PEStrings = extractASCIIStrings(blob, 6, 400) // cap to keep memory sane
			ev.Kind = classifyBootloaderKind(p, ev.Sections, ev.PEStrings, ev.HasSBAT)
		}
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

func extractASCIIStrings(b []byte, minLen int, maxStrings int) []string {
	out := make([]string, 0, 64)

	start := -1
	for i := 0; i < len(b); i++ {
		c := b[i]
		// printable ASCII range (space..~)
		if c >= 0x20 && c <= 0x7e {
			if start == -1 {
				start = i
			}
			continue
		}

		if start != -1 {
			if i-start >= minLen {
				s := string(b[start:i])
				out = append(out, s)
				if maxStrings > 0 && len(out) >= maxStrings {
					return out
				}
			}
			start = -1
		}
	}

	// tail
	if start != -1 && len(b)-start >= minLen {
		out = append(out, string(b[start:]))
	}

	return out
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

func classifyBootloaderKind(p string, sections []string, peStrings []string, hasSBAT bool) BootloaderKind {
	lp := strings.ToLower(p)

	// Deterministic first:
	if sections != nil && hasSection(sections, ".linux") {
		return BootloaderUKI
	}

	// Path / filename heuristics:
	if strings.Contains(lp, "mmx64.efi") || strings.Contains(lp, "mmia32.efi") {
		return BootloaderMokManager
	}
	if strings.Contains(lp, "shim") {
		return BootloaderShim
	}
	if strings.Contains(lp, "systemd") && strings.Contains(lp, "boot") {
		return BootloaderSystemdBoot
	}
	if strings.Contains(lp, "grub") {
		return BootloaderGrub
	}

	// Content / metadata hints (for BOOTX64.EFI etc.)
	if hasSBAT {
		// Many shim builds have .sbat; GRUB can too, so treat as weak signal unless strings confirm.
		// We'll fall through to strings below.
	}

	// String fingerprint fallback
	// (Keep it conservative: only return a kind when we see strong markers.)
	if len(peStrings) > 0 {
		contains := func(needle string) bool {
			needle = strings.ToLower(needle)
			for _, s := range peStrings {
				if strings.Contains(strings.ToLower(s), needle) {
					return true
				}
			}
			return false
		}

		// GRUB
		if contains("grub") && (contains("normal.mod") || contains("grub.cfg") || contains("configfile")) {
			return BootloaderGrub
		}
		// systemd-boot
		if contains("systemd-boot") || contains("loaderimageidentifier") {
			return BootloaderSystemdBoot
		}
		// shim / MokManager
		if contains("mokmanager") || contains("machine owner key") {
			return BootloaderMokManager
		}
		if contains("shim") || (hasSBAT && contains("sbat")) {
			return BootloaderShim
		}
	}

	return BootloaderUnknown
}

func containsAny(hay []string, needles ...string) bool {
	for _, s := range hay {
		for _, n := range needles {
			if strings.Contains(s, n) {
				return true
			}
		}
	}
	return false
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
