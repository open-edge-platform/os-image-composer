package imageinspect

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

// FAT filesystem reader implementation (for raw reads from disk images)
type fatKind int

// FAT kinds
const (
	fatUnknown fatKind = iota
	fat12
	fat16
	fat32
)

// fatVol represents an opened FAT volume.
type fatVol struct {
	r       io.ReaderAt
	baseOff int64 // partition start offset in bytes

	kind fatKind

	// BPB common
	bytsPerSec uint16
	secPerClus uint8
	rsvdSecCnt uint16
	numFATs    uint8
	rootEntCnt uint16
	totSec     uint32

	// FAT16
	fatSz16 uint16

	// FAT32
	fatSz32  uint32
	rootClus uint32

	// derived
	fatStart       int64
	rootDirStart   int64 // FAT16 fixed root
	rootDirSectors uint32
	dataStart      int64
	clusterSize    uint32
}

// fatDirEntry represents a directory entry in a FAT filesystem.
type fatDirEntry struct {
	name         string
	isDir        bool
	firstCluster uint32
	size         uint32
}

// scanAndHashEFIFromRawFAT scans the FAT filesystem at the given partition offset
// within the provided ReaderAt, looking for EFI binaries under /EFI, hashing them,
// and populating the provided FilesystemSummary with the findings.
func scanAndHashEFIFromRawFAT(r io.ReaderAt, partOff int64, out *FilesystemSummary) error {
	v, err := openFAT(r, partOff)
	if err != nil {
		return err
	}

	if _, err := v.findPath("EFI"); err != nil {
		return nil
	}

	type item struct{ dir string }
	stack := []item{{dir: "EFI"}}

	var (
		hasShim bool
		hasUKI  bool
	)

	out.EFIBinaries = nil

	seen := map[string]struct{}{} // optional dedupe

	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		ents, err := v.listDir(cur.dir)
		if err != nil {
			continue
		}

		for _, de := range ents {
			full := path.Join(cur.dir, de.name)

			if de.isDir {
				stack = append(stack, item{dir: full})
				continue
			}

			nameLower := strings.ToLower(de.name)
			if !strings.HasSuffix(nameLower, ".efi") {
				continue
			}

			fullLower := strings.ToLower(full)
			if _, ok := seen[fullLower]; ok {
				continue
			}
			seen[fullLower] = struct{}{}

			if strings.HasPrefix(fullLower, "efi/linux/") {
				hasUKI = true
			}
			if strings.Contains(nameLower, "shim") {
				hasShim = true
			}

			b, sz, err := v.readFileByEntry(&de)
			if err != nil {
				out.Notes = append(out.Notes, fmt.Sprintf("read %s failed: %v", full, err))
				continue
			}

			peEv, err := ParsePEFromBytes(full, b)
			if err != nil {
				out.Notes = append(out.Notes, fmt.Sprintf("PE parse %s failed: %v", full, err))
				continue
			}

			peEv.Size = sz
			out.EFIBinaries = append(out.EFIBinaries, peEv)
		}
	}

	sort.Slice(out.EFIBinaries, func(i, j int) bool { return out.EFIBinaries[i].Path < out.EFIBinaries[j].Path })

	out.HasShim = out.HasShim || hasShim
	out.HasUKI = out.HasUKI || hasUKI
	return nil
}

// sha256Hex returns the SHA256 hash of the given byte slice as a hex string.
func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// readFileByEntry reads the contents of the file represented by the given fatDirEntry.
func (v *fatVol) readFileByEntry(e *fatDirEntry) ([]byte, int64, error) {
	remaining := int64(e.size)
	var out []byte
	out = make([]byte, 0, remaining)

	c := e.firstCluster
	seen := map[uint32]bool{}

	for c >= 2 && !v.isEOC(c) && remaining > 0 {
		if seen[c] {
			return nil, 0, fmt.Errorf("FAT loop detected at cluster %d", c)
		}
		seen[c] = true

		off := v.clusterOff(c)
		chunk := make([]byte, v.clusterSize)
		if _, err := v.r.ReadAt(chunk, off); err != nil && err != io.EOF {
			return nil, 0, err
		}

		n := int64(len(chunk))
		if remaining < n {
			n = remaining
		}
		out = append(out, chunk[:n]...)
		remaining -= n

		next, err := v.fatEntry(c)
		if err != nil {
			return nil, 0, err
		}
		c = next
	}

	return out, int64(e.size), nil
}

// listDir lists the directory entries at the given path within the FAT volume.
func (v *fatVol) listDir(dir string) ([]fatDirEntry, error) {
	dir = strings.Trim(dir, "/")
	if dir == "" {
		return v.readRootDir()
	}

	e, err := v.findPath(dir)
	if err != nil {
		return nil, err
	}
	if !e.isDir {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}

	return v.readDirFromCluster(e.firstCluster)
}

// findPath finds the directory entry for the given path within the FAT volume.
func (v *fatVol) findPath(p string) (*fatDirEntry, error) {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil, fmt.Errorf("empty path")
	}
	parts := strings.Split(p, "/")

	ents, err := v.readRootDir()
	if err != nil {
		return nil, err
	}

	for i, part := range parts {
		var match *fatDirEntry
		for _, e := range ents {
			if strings.EqualFold(e.name, part) {
				tmp := e
				match = &tmp
				break
			}
		}
		if match == nil {
			return nil, os.ErrNotExist
		}
		if i == len(parts)-1 {
			return match, nil
		}
		if !match.isDir {
			return nil, fmt.Errorf("not a directory: %s", part)
		}
		ents, err = v.readDirFromCluster(match.firstCluster)
		if err != nil {
			return nil, err
		}
	}

	return nil, os.ErrNotExist
}

// readDirFromCluster reads directory entries starting from the given cluster.
func (v *fatVol) readDirFromCluster(startCluster uint32) ([]fatDirEntry, error) {
	var all []byte
	c := startCluster
	seen := map[uint32]bool{}

	for c >= 2 && !v.isEOC(c) {
		if seen[c] {
			return nil, fmt.Errorf("FAT loop detected at cluster %d", c)
		}
		seen[c] = true

		off := v.clusterOff(c)
		chunk := make([]byte, v.clusterSize)
		if _, err := v.r.ReadAt(chunk, off); err != nil && err != io.EOF {
			return nil, err
		}
		all = append(all, chunk...)

		next, err := v.fatEntry(c)
		if err != nil {
			return nil, err
		}
		c = next
	}

	return parseDirEntries(all)
}

// parseDirEntries parses raw directory entry bytes into a slice of fatDirEntry.
func parseDirEntries(buf []byte) ([]fatDirEntry, error) {
	var out []fatDirEntry
	var lfnParts []string

	for off := 0; off+32 <= len(buf); off += 32 {
		e := buf[off : off+32]
		if e[0] == 0x00 {
			break
		}
		if e[0] == 0xE5 {
			lfnParts = nil
			continue
		}

		attr := e[11]
		if attr == 0x0F {
			part := decodeLFNPart(e)
			if part != "" {
				lfnParts = append(lfnParts, part)
			}
			continue
		}

		// volume label entry?
		if attr&0x08 != 0 {
			lfnParts = nil
			continue
		}

		name := ""
		if len(lfnParts) > 0 {
			for i, j := 0, len(lfnParts)-1; i < j; i, j = i+1, j-1 {
				lfnParts[i], lfnParts[j] = lfnParts[j], lfnParts[i]
			}
			name = strings.Join(lfnParts, "")
		} else {
			name = decode83Name(e[0:11])
		}
		lfnParts = nil

		isDir := (attr & 0x10) != 0

		// FAT32 stores high 16 bits in e[20:22]
		clusHi := binary.LittleEndian.Uint16(e[20:22])
		clusLo := binary.LittleEndian.Uint16(e[26:28])
		firstClus := (uint32(clusHi) << 16) | uint32(clusLo)

		size := binary.LittleEndian.Uint32(e[28:32])

		if name == "." || name == ".." {
			continue
		}

		out = append(out, fatDirEntry{
			name:         name,
			isDir:        isDir,
			firstCluster: firstClus,
			size:         size,
		})
	}

	return out, nil
}

// readRootDir reads the root directory entries of the FAT volume.
func (v *fatVol) readRootDir() ([]fatDirEntry, error) {
	if v.kind == fat32 {
		return v.readDirFromCluster(v.rootClus)
	}

	// FAT16 root is fixed region
	sizeBytes := int64(v.rootDirSectors) * int64(v.bytsPerSec)
	buf := make([]byte, sizeBytes)
	if _, err := v.r.ReadAt(buf, v.rootDirStart); err != nil && err != io.EOF {
		return nil, err
	}
	return parseDirEntries(buf)
}

// decode83Name decodes an 8.3 filename from a byte slice.
func decode83Name(b []byte) string {
	base := strings.TrimRight(string(b[0:8]), " ")
	ext := strings.TrimRight(string(b[8:11]), " ")
	if ext != "" {
		return base + "." + ext
	}
	return base
}

// decodeLFNPart decodes a single LFN part from a directory entry.
func decodeLFNPart(e []byte) string {
	// 13 UTF-16LE chars in 3 ranges
	chars := make([]uint16, 0, 13)
	readU16 := func(i int) uint16 { return binary.LittleEndian.Uint16(e[i : i+2]) }

	for _, i := range []int{1, 3, 5, 7, 9} {
		chars = append(chars, readU16(i))
	}
	for _, i := range []int{14, 16, 18, 20, 22, 24} {
		chars = append(chars, readU16(i))
	}
	for _, i := range []int{28, 30} {
		chars = append(chars, readU16(i))
	}

	// convert until 0x0000 or 0xFFFF
	var sb strings.Builder
	for _, c := range chars {
		if c == 0x0000 || c == 0xFFFF {
			break
		}
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

// isEOC checks if the given cluster number indicates end-of-chain.
func (v *fatVol) isEOC(c uint32) bool {
	switch v.kind {
	case fat32:
		return c >= 0x0FFFFFF8
	default: // fat16 (and fat12 if you ever add)
		return c >= 0xFFF8
	}
}

// clusterOff returns the byte offset of the given cluster within the FAT volume.
func (v *fatVol) clusterOff(cluster uint32) int64 {
	// data clusters start at 2
	if cluster < 2 {
		return v.dataStart
	}
	dataClusterIndex := cluster - 2
	return v.dataStart + int64(dataClusterIndex)*int64(v.clusterSize)
}

// fatEntry reads the FAT entry for the given cluster number.
func (v *fatVol) fatEntry(cluster uint32) (uint32, error) {
	switch v.kind {
	case fat32:
		off := v.fatStart + int64(cluster)*4
		b := make([]byte, 4)
		if _, err := v.r.ReadAt(b, off); err != nil && err != io.EOF {
			return 0, err
		}
		// FAT32 uses only low 28 bits
		return binary.LittleEndian.Uint32(b) & 0x0FFFFFFF, nil
	case fat12:
		// optional: implement later; ESP won’t be FAT12
		return 0, fmt.Errorf("FAT12 not supported")
	default: // fat16
		off := v.fatStart + int64(cluster)*2
		b := make([]byte, 2)
		if _, err := v.r.ReadAt(b, off); err != nil && err != io.EOF {
			return 0, err
		}
		return uint32(binary.LittleEndian.Uint16(b)), nil
	}
}

// openFAT parses BPB, classifies FAT12/16/32, and fills derived layout offsets.
func openFAT(r io.ReaderAt, baseOff int64) (*fatVol, error) {
	bs := make([]byte, 512)
	if _, err := r.ReadAt(bs, baseOff); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read boot sector: %w", err)
	}
	if bs[510] != 0x55 || bs[511] != 0xAA {
		return nil, fmt.Errorf("invalid boot sector signature")
	}

	v := &fatVol{r: r, baseOff: baseOff}

	// BPB common
	v.bytsPerSec = binary.LittleEndian.Uint16(bs[11:13])
	v.secPerClus = bs[13]
	v.rsvdSecCnt = binary.LittleEndian.Uint16(bs[14:16])
	v.numFATs = bs[16]
	v.rootEntCnt = binary.LittleEndian.Uint16(bs[17:19])

	totSec16 := binary.LittleEndian.Uint16(bs[19:21])
	v.fatSz16 = binary.LittleEndian.Uint16(bs[22:24])
	totSec32 := binary.LittleEndian.Uint32(bs[32:36])

	v.totSec = uint32(totSec16)
	if v.totSec == 0 {
		v.totSec = totSec32
	}

	if v.bytsPerSec == 0 || v.secPerClus == 0 || v.rsvdSecCnt == 0 || v.numFATs == 0 {
		return nil, fmt.Errorf("invalid BPB fields")
	}
	v.clusterSize = uint32(v.bytsPerSec) * uint32(v.secPerClus)

	// FAT32 fields (only meaningful for FAT32)
	v.fatSz32 = binary.LittleEndian.Uint32(bs[36:40])
	v.rootClus = binary.LittleEndian.Uint32(bs[44:48])

	// Derived layout
	v.fatStart = v.baseOff + int64(v.rsvdSecCnt)*int64(v.bytsPerSec)

	// Determine FAT32 vs FAT12/16 using canonical BPB conditions
	isFAT32 := (v.rootEntCnt == 0) && (v.fatSz16 == 0) && (v.fatSz32 != 0)

	if isFAT32 {
		v.kind = fat32
		// FAT32: root directory is a cluster chain starting at rootClus
		// rootDirStart/rootDirSectors are unused in FAT32 (keep 0)
		v.dataStart = v.fatStart + int64(v.numFATs)*int64(v.fatSz32)*int64(v.bytsPerSec)
		return v, nil
	}

	// FAT12/16: need cluster count to classify accurately
	if v.fatSz16 == 0 {
		return nil, fmt.Errorf("invalid FAT16 BPB: fatSz16=0 and not FAT32")
	}

	v.rootDirSectors = ((uint32(v.rootEntCnt) * 32) + (uint32(v.bytsPerSec) - 1)) / uint32(v.bytsPerSec)
	v.rootDirStart = v.fatStart + int64(v.numFATs)*int64(v.fatSz16)*int64(v.bytsPerSec)
	v.dataStart = v.rootDirStart + int64(v.rootDirSectors)*int64(v.bytsPerSec)

	// Data sectors:
	dataSectors := v.totSec - (uint32(v.rsvdSecCnt) + (uint32(v.numFATs) * uint32(v.fatSz16)) + v.rootDirSectors)
	clusterCount := dataSectors / uint32(v.secPerClus)

	switch {
	case clusterCount < 4085:
		v.kind = fat12
	case clusterCount < 65525:
		v.kind = fat16
	default:
		// rare but possible; treat as FAT32-ish, but BPB wasn't FAT32; keep fat16 to avoid misreading FAT entries.
		v.kind = fat16
	}

	return v, nil
}
