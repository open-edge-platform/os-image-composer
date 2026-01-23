package imageinspect

import (
	"fmt"
	"sort"
	"strings"
)

// ImageCompareResult represents the result of comparing two images.
type ImageCompareResult struct {
	SchemaVersion string `json:"schemaVersion,omitempty"`

	From ImageSummary `json:"from"`
	To   ImageSummary `json:"to"`

	Equal bool `json:"equal"`

	Summary CompareSummary `json:"summary,omitempty"`
	Diff    ImageDiff      `json:"diff,omitempty"`
}

// CompareSummary provides a high-level summary of differences between two images.
type CompareSummary struct {
	Changed bool `json:"changed,omitempty"`

	PartitionTableChanged bool `json:"partitionTableChanged,omitempty"`
	PartitionsChanged     bool `json:"partitionsChanged,omitempty"`
	FilesystemsChanged    bool `json:"filesystemsChanged,omitempty"`
	EFIBinariesChanged    bool `json:"efiBinariesChanged,omitempty"`

	AddedCount    int `json:"addedCount,omitempty"`
	RemovedCount  int `json:"removedCount,omitempty"`
	ModifiedCount int `json:"modifiedCount,omitempty"`
}

// ImageDiff represents the differences between two ImageSummary objects.
type ImageDiff struct {
	Image          MetaDiff           `json:"image,omitempty"`
	PartitionTable PartitionTableDiff `json:"partitionTable,omitempty"`
	Partitions     PartitionDiff      `json:"partitions,omitempty"`
	EFIBinaries    EFIBinaryDiff      `json:"efiBinaries,omitempty"`
}

// MetaDiff represents differences in image-level metadata.
type MetaDiff struct {
	File      *ValueDiff[string] `json:"file,omitempty"`
	SizeBytes *ValueDiff[int64]  `json:"sizeBytes,omitempty"`
}

// PartitionTableDiff represents differences in partition table-level fields.
type PartitionTableDiff struct {
	Type               *ValueDiff[string] `json:"type,omitempty"`
	LogicalSectorSize  *ValueDiff[int64]  `json:"logicalSectorSize,omitempty"`
	PhysicalSectorSize *ValueDiff[int64]  `json:"physicalSectorSize,omitempty"`
	ProtectiveMBR      *ValueDiff[bool]   `json:"protectiveMbr,omitempty"`

	Changed bool `json:"changed,omitempty"`
}

type ValueDiff[T any] struct {
	From T `json:"from"`
	To   T `json:"to"`
}

// FieldChange represents a change in a single field between two objects.
type FieldChange struct {
	Field string `json:"field"`
	From  any    `json:"from,omitempty"`
	To    any    `json:"to,omitempty"`
}

// PartitionDiff represents added, removed, and modified partitions.
type PartitionDiff struct {
	Added    []PartitionSummary         `json:"added,omitempty"`
	Removed  []PartitionSummary         `json:"removed,omitempty"`
	Modified []ModifiedPartitionSummary `json:"modified,omitempty"`
}

// ModifiedPartitionSummary represents changes between two PartitionSummary objects.
type ModifiedPartitionSummary struct {
	Key     string           `json:"key"`
	From    PartitionSummary `json:"from"`
	To      PartitionSummary `json:"to"`
	Changes []FieldChange    `json:"changes,omitempty"`

	Filesystem  *FilesystemChange `json:"filesystem,omitempty"`
	EFIBinaries *EFIBinaryDiff    `json:"efiBinaries,omitempty"`
}

// Filesystem changes
type FilesystemChange struct {
	Added    *FilesystemSummary         `json:"added,omitempty"`
	Removed  *FilesystemSummary         `json:"removed,omitempty"`
	Modified *ModifiedFilesystemSummary `json:"modified,omitempty"`
}

// ModifiedFilesystemSummary represents changes between two FilesystemSummary objects.
type ModifiedFilesystemSummary struct {
	From    FilesystemSummary `json:"from"`
	To      FilesystemSummary `json:"to"`
	Changes []FieldChange     `json:"changes,omitempty"`
}

// EFI binaries
type EFIBinaryDiff struct {
	Added    []EFIBinaryEvidence         `json:"added,omitempty"`
	Removed  []EFIBinaryEvidence         `json:"removed,omitempty"`
	Modified []ModifiedEFIBinaryEvidence `json:"modified,omitempty"`
}

// ModifiedEFIBinaryEvidence represents a modified EFI binary evidence entry.
type ModifiedEFIBinaryEvidence struct {
	Key     string            `json:"key"`
	From    EFIBinaryEvidence `json:"from"`
	To      EFIBinaryEvidence `json:"to"`
	Changes []FieldChange     `json:"changes,omitempty"`

	UKI *UKIDiff `json:"uki,omitempty"`
}

// UKIDiff represents differences in the UKI-related fields of an EFI binary.
type UKIDiff struct {
	KernelSHA256  *ValueDiff[string] `json:"kernelSha256,omitempty"`
	InitrdSHA256  *ValueDiff[string] `json:"initrdSha256,omitempty"`
	CmdlineSHA256 *ValueDiff[string] `json:"cmdlineSha256,omitempty"`
	OSRelSHA256   *ValueDiff[string] `json:"osrelSha256,omitempty"`
	UnameSHA256   *ValueDiff[string] `json:"unameSha256,omitempty"`

	SectionSHA256 SectionMapDiff `json:"sectionSha256,omitempty"`

	Changed bool `json:"changed,omitempty"`
}

// SectionMapDiff represents differences in a map of section names to their SHA256 hashes.
type SectionMapDiff struct {
	Added    map[string]string            `json:"added,omitempty"`
	Removed  map[string]string            `json:"removed,omitempty"`
	Modified map[string]ValueDiff[string] `json:"modified,omitempty"`
}

// CompareImages compares two ImageSummary objects and returns a structured diff.
// The caller can JSON-marshal the returned result.
func CompareImages(from, to *ImageSummary) ImageCompareResult {

	if from == nil || to == nil {
		return ImageCompareResult{Equal: false}
	}
	res := ImageCompareResult{
		SchemaVersion: "1",
		From:          *from,
		To:            *to,
		Equal:         true,
	}

	// --- image meta ---
	res.Diff.Image = compareMeta(*from, *to)
	if res.Diff.Image.File != nil || res.Diff.Image.SizeBytes != nil {
		res.Summary.ModifiedCount++
		res.Summary.Changed = true
		// NOTE: we may choose NOT to treat filename changes as "unequal".
		res.Equal = false
	}

	// --- partition table (table-level fields) ---
	res.Diff.PartitionTable = comparePartitionTable(from.PartitionTable, to.PartitionTable)
	if res.Diff.PartitionTable.Changed {
		res.Summary.PartitionTableChanged = true
		res.Summary.Changed = true
		res.Equal = false
	}

	// --- partitions (including filesystem changes and per-partition EFI evidence diffs) ---
	res.Diff.Partitions = comparePartitions(from.PartitionTable, to.PartitionTable)
	if len(res.Diff.Partitions.Added) > 0 || len(res.Diff.Partitions.Removed) > 0 || len(res.Diff.Partitions.Modified) > 0 {
		res.Summary.PartitionsChanged = true
		res.Summary.Changed = true
		res.Equal = false
		res.Summary.AddedCount += len(res.Diff.Partitions.Added)
		res.Summary.RemovedCount += len(res.Diff.Partitions.Removed)
		res.Summary.ModifiedCount += len(res.Diff.Partitions.Modified)
	}

	// --- global roll-up of EFI evidence across all partitions/filesystems ---
	res.Diff.EFIBinaries = compareEFIBinaries(flattenEFIBinaries(from.PartitionTable), flattenEFIBinaries(to.PartitionTable))
	if len(res.Diff.EFIBinaries.Added) > 0 || len(res.Diff.EFIBinaries.Removed) > 0 || len(res.Diff.EFIBinaries.Modified) > 0 {
		res.Summary.EFIBinariesChanged = true
		res.Summary.Changed = true
		res.Equal = false
	}

	// Deterministic ordering for stable JSON
	normalizeCompareResult(&res)

	return res
}

func compareMeta(from, to ImageSummary) MetaDiff {
	var out MetaDiff
	if from.File != to.File {
		out.File = &ValueDiff[string]{From: from.File, To: to.File}
	}
	if from.SizeBytes != to.SizeBytes {
		out.SizeBytes = &ValueDiff[int64]{From: from.SizeBytes, To: to.SizeBytes}
	}
	return out
}

// comparePartitionTable compares two PartitionTableSummary objects and returns a PartitionTableDiff.
func comparePartitionTable(from, to PartitionTableSummary) PartitionTableDiff {
	var d PartitionTableDiff

	if from.Type != to.Type {
		d.Type = &ValueDiff[string]{From: from.Type, To: to.Type}
	}
	if from.LogicalSectorSize != to.LogicalSectorSize {
		d.LogicalSectorSize = &ValueDiff[int64]{From: from.LogicalSectorSize, To: to.LogicalSectorSize}
	}
	if from.PhysicalSectorSize != to.PhysicalSectorSize {
		d.PhysicalSectorSize = &ValueDiff[int64]{From: from.PhysicalSectorSize, To: to.PhysicalSectorSize}
	}
	if from.ProtectiveMBR != to.ProtectiveMBR {
		d.ProtectiveMBR = &ValueDiff[bool]{From: from.ProtectiveMBR, To: to.ProtectiveMBR}
	}

	d.Changed = d.Type != nil || d.LogicalSectorSize != nil || d.PhysicalSectorSize != nil || d.ProtectiveMBR != nil
	return d
}

// comparePartitions compares two PartitionTableSummary objects and returns a PartitionDiff.
func comparePartitions(fromPT, toPT PartitionTableSummary) PartitionDiff {
	fromParts := indexPartitions(fromPT)
	toParts := indexPartitions(toPT)

	out := PartitionDiff{}

	// Keys union
	keys := make([]string, 0, len(fromParts)+len(toParts))
	seen := map[string]struct{}{}
	for k := range fromParts {
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for k := range toParts {
		if _, ok := seen[k]; !ok {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	for _, k := range keys {
		f, fok := fromParts[k]
		t, tok := toParts[k]

		switch {
		case fok && !tok:
			out.Removed = append(out.Removed, f)
		case !fok && tok:
			out.Added = append(out.Added, t)
		case fok && tok:
			if partitionsEqual(f, t) {
				continue
			}

			mod := ModifiedPartitionSummary{
				Key:  k,
				From: f,
				To:   t,
			}

			// Optional human-friendly changes (keep minimal, add more later)
			mod.Changes = appendPartitionFieldChanges(nil, f, t)

			// Filesystem changes
			mod.Filesystem = compareFilesystemPtrs(f.Filesystem, t.Filesystem)

			// Per-partition EFI evidence diff if there is filesystem evidence on either side
			fEFI := flattenEFIBinariesFromPartition(f)
			tEFI := flattenEFIBinariesFromPartition(t)
			if len(fEFI) > 0 || len(tEFI) > 0 {
				efiDiff := compareEFIBinaries(fEFI, tEFI)
				// Only include if something actually changed
				if len(efiDiff.Added) > 0 || len(efiDiff.Removed) > 0 || len(efiDiff.Modified) > 0 {
					mod.EFIBinaries = &efiDiff
				}
			}

			out.Modified = append(out.Modified, mod)
		}
	}

	return out
}

func indexPartitions(pt PartitionTableSummary) map[string]PartitionSummary {
	out := make(map[string]PartitionSummary, len(pt.Partitions))

	for _, p := range pt.Partitions {
		key := partitionKey(pt.Type, p)

		// Ensure uniqueness even if names collide (rare but possible)
		if _, exists := out[key]; exists {
			key = fmt.Sprintf("%s#idx=%d", key, p.Index)
			if _, exists2 := out[key]; exists2 {
				key = fmt.Sprintf("%s#lba=%d-%d", key, p.StartLBA, p.EndLBA)
			}
		}

		out[key] = p
	}
	return out
}

func partitionKey(ptType string, p PartitionSummary) string {
	ptType = strings.ToLower(strings.TrimSpace(ptType))
	name := strings.ToLower(strings.TrimSpace(p.Name))

	// normalize empty-ish name
	if name == "" {
		name = "-"
	}

	switch ptType {
	case "gpt":
		// GPT type GUID is the strongest “role identity”.
		t := strings.ToUpper(strings.TrimSpace(p.Type))
		if t != "" {
			return fmt.Sprintf("gpt:%s:%s", t, name)
		}
		// fallback: name + index
		return fmt.Sprintf("gpt:%s:idx=%d", name, p.Index)

	case "mbr":
		t := strings.ToLower(strings.TrimSpace(p.Type)) // like 0x0c, 0x83
		if t == "" {
			t = "unknown"
		}
		// MBR doesn’t have GUID roles; index + type is usually stable enough
		return fmt.Sprintf("mbr:%s:%s:idx=%d", t, name, p.Index)

	default:
		// unknown PT: best effort
		t := strings.ToLower(strings.TrimSpace(p.Type))
		if t == "" {
			t = "unknown"
		}
		return fmt.Sprintf("pt:%s:%s:idx=%d", t, name, p.Index)
	}
}

// partitionsEqual checks if two PartitionSummary objects are equal.
func partitionsEqual(a, b PartitionSummary) bool {
	
	if a.Index != b.Index ||
		a.Name != b.Name ||
		a.Type != b.Type ||
		a.StartLBA != b.StartLBA ||
		a.EndLBA != b.EndLBA ||
		a.SizeBytes != b.SizeBytes ||
		a.Flags != b.Flags ||
		a.LogicalSectorSize != b.LogicalSectorSize {
		return false
	}

	// Filesystem pointer nil-ness / equality
	return filesystemPtrsEqual(a.Filesystem, b.Filesystem)
}

func filesystemPtrsEqual(a, b *FilesystemSummary) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return filesystemEqual(*a, *b)
}

func filesystemEqual(a, b FilesystemSummary) bool {
	// Compare high-value fields
	if a.Type != b.Type ||
		a.Label != b.Label ||
		a.UUID != b.UUID ||
		a.BlockSize != b.BlockSize ||
		a.HasShim != b.HasShim ||
		a.HasUKI != b.HasUKI ||
		a.FATType != b.FATType ||
		a.BytesPerSector != b.BytesPerSector ||
		a.SectorsPerCluster != b.SectorsPerCluster ||
		a.ClusterCount != b.ClusterCount ||
		a.Compression != b.Compression ||
		a.Version != b.Version {
		return false
	}

	// Slice comparisons (sorted for determinism)
	if !stringSliceEqualSorted(a.Features, b.Features) {
		return false
	}
	if !stringSliceEqualSorted(a.Notes, b.Notes) {
		return false
	}
	if !stringSliceEqualSorted(a.FsFlags, b.FsFlags) {
		return false
	}

	// EFI binaries evidence list
	return efiEvidenceListEqual(a.EFIBinaries, b.EFIBinaries)
}

func stringSliceEqualSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a2 := append([]string(nil), a...)
	b2 := append([]string(nil), b...)
	sort.Strings(a2)
	sort.Strings(b2)
	for i := range a2 {
		if a2[i] != b2[i] {
			return false
		}
	}
	return true
}

func efiEvidenceListEqual(a, b []EFIBinaryEvidence) bool {
	if len(a) != len(b) {
		return false
	}
	// Compare by path (stable key)
	am := make(map[string]EFIBinaryEvidence, len(a))
	for _, e := range a {
		am[e.Path] = e
	}
	for _, e := range b {
		ae, ok := am[e.Path]
		if !ok {
			return false
		}
		if !efiEvidenceEqual(ae, e) {
			return false
		}
	}
	return true
}

func efiEvidenceEqual(a, b EFIBinaryEvidence) bool {

	// Compare high-value evidence. 
	if a.Path != b.Path ||
		a.Size != b.Size ||
		a.SHA256 != b.SHA256 ||
		a.Arch != b.Arch ||
		a.Kind != b.Kind ||
		a.Signed != b.Signed ||
		a.SignatureSize != b.SignatureSize ||
		a.HasSBAT != b.HasSBAT ||
		a.IsUKI != b.IsUKI ||
		a.KernelSHA256 != b.KernelSHA256 ||
		a.InitrdSHA256 != b.InitrdSHA256 ||
		a.CmdlineSHA256 != b.CmdlineSHA256 ||
		a.OSRelSHA256 != b.OSRelSHA256 ||
		a.UnameSHA256 != b.UnameSHA256 {
		return false
	}

	if !stringSliceEqualSorted(a.Sections, b.Sections) {
		return false
	}

	// SectionSHA256 map compare
	if !stringMapEqual(a.SectionSHA256, b.SectionSHA256) {
		return false
	}

	// Notes not super important, but include if you want exactness
	if !stringSliceEqualSorted(a.Notes, b.Notes) {
		return false
	}

	// OSRelease map compare (optional)
	if !stringMapEqual(a.OSRelease, b.OSRelease) {
		return false
	}

	return true
}

func stringMapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if bv, ok := b[k]; !ok || bv != av {
			return false
		}
	}
	return true
}

func appendPartitionFieldChanges(dst []FieldChange, a, b PartitionSummary) []FieldChange {
	add := func(field string, from, to any) {
		dst = append(dst, FieldChange{Field: field, From: from, To: to})
	}

	if a.Index != b.Index {
		add("index", a.Index, b.Index)
	}
	if a.Name != b.Name {
		add("name", a.Name, b.Name)
	}
	if a.Type != b.Type {
		add("type", a.Type, b.Type)
	}
	if a.StartLBA != b.StartLBA {
		add("startLBA", a.StartLBA, b.StartLBA)
	}
	if a.EndLBA != b.EndLBA {
		add("endLBA", a.EndLBA, b.EndLBA)
	}
	if a.SizeBytes != b.SizeBytes {
		add("sizeBytes", a.SizeBytes, b.SizeBytes)
	}
	if a.Flags != b.Flags {
		add("flags", a.Flags, b.Flags)
	}
	if a.LogicalSectorSize != b.LogicalSectorSize {
		add("logicalSectorSize", a.LogicalSectorSize, b.LogicalSectorSize)
	}

	return dst
}

func compareFilesystemPtrs(a, b *FilesystemSummary) *FilesystemChange {
	if a == nil && b == nil {
		return nil
	}
	if a == nil && b != nil {
		return &FilesystemChange{Added: b}
	}
	if a != nil && b == nil {
		return &FilesystemChange{Removed: a}
	}
	if a != nil && b != nil && filesystemEqual(*a, *b) {
		return nil
	}

	out := &FilesystemChange{
		Modified: &ModifiedFilesystemSummary{
			From: *a,
			To:   *b,
		},
	}
	out.Modified.Changes = appendFilesystemFieldChanges(nil, *a, *b)
	return out
}

func appendFilesystemFieldChanges(dst []FieldChange, a, b FilesystemSummary) []FieldChange {
	add := func(field string, from, to any) {
		dst = append(dst, FieldChange{Field: field, From: from, To: to})
	}
	if a.Type != b.Type {
		add("type", a.Type, b.Type)
	}
	if a.Label != b.Label {
		add("label", a.Label, b.Label)
	}
	if a.UUID != b.UUID {
		add("uuid", a.UUID, b.UUID)
	}
	if a.HasShim != b.HasShim {
		add("hasShim", a.HasShim, b.HasShim)
	}
	if a.HasUKI != b.HasUKI {
		add("hasUki", a.HasUKI, b.HasUKI)
	}
	return dst
}
