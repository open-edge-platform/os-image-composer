package imageinspect

import (
	"testing"
)

func TestCompareImages_Equal_NoChanges(t *testing.T) {
	a := &ImageSummary{
		File:      "a.raw",
		SizeBytes: 100,
		PartitionTable: PartitionTableSummary{
			Type:               "gpt",
			LogicalSectorSize:  512,
			PhysicalSectorSize: 4096,
			ProtectiveMBR:      true,
			Partitions: []PartitionSummary{
				{
					Index:    1,
					Name:     "ESP",
					Type:     "efi",
					StartLBA:  2048,
					EndLBA:    4095,
					SizeBytes: 1024,
					Flags:     "",
					Filesystem: &FilesystemSummary{
						Type:  "vfat",
						Label: "EFI",
						UUID:  "UUID-ESP",
					},
				},
			},
		},
	}

	// Deep copy-ish by constructing again (enough for this test)
	b := &ImageSummary{
		File:      "a.raw",
		SizeBytes: 100,
		PartitionTable: a.PartitionTable,
	}

	res := CompareImages(a, b)

	// Depending on your policy, Equal might ignore filename differences.
	// If your CompareImages treats File diff as non-semantic, Equal should be true.
	// If it treats it as semantic, update this assertion.
	if res.Equal != true {
		t.Fatalf("expected Equal=true, got %v", res.Equal)
	}
	if res.Summary.Changed {
		t.Fatalf("expected Summary.Changed=false, got true")
	}
	if res.Diff.PartitionTable.Changed {
		t.Fatalf("expected no partition table change")
	}
	if len(res.Diff.Partitions.Added) != 0 || len(res.Diff.Partitions.Removed) != 0 || len(res.Diff.Partitions.Modified) != 0 {
		t.Fatalf("expected no partition changes")
	}
	if len(res.Diff.EFIBinaries.Added) != 0 || len(res.Diff.EFIBinaries.Removed) != 0 || len(res.Diff.EFIBinaries.Modified) != 0 {
		t.Fatalf("expected no efi changes")
	}
}

func TestCompareImages_PartitionTableChanged(t *testing.T) {
	a := &ImageSummary{
		File:      "a.raw",
		SizeBytes: 100,
		PartitionTable: PartitionTableSummary{
			Type:               "gpt",
			LogicalSectorSize:  512,
			PhysicalSectorSize: 4096,
			ProtectiveMBR:      true,
		},
	}
	b := &ImageSummary{
		File:      "b.raw",
		SizeBytes: 100,
		PartitionTable: PartitionTableSummary{
			Type:               "mbr",
			LogicalSectorSize:  512,
			PhysicalSectorSize: 512,
			ProtectiveMBR:      false,
		},
	}

	res := CompareImages(a, b)
	if res.Equal {
		t.Fatalf("expected Equal=false")
	}
	if !res.Diff.PartitionTable.Changed {
		t.Fatalf("expected partition table changed")
	}
	if res.Diff.PartitionTable.Type == nil || res.Diff.PartitionTable.Type.From != "gpt" || res.Diff.PartitionTable.Type.To != "mbr" {
		t.Fatalf("expected type diff gpt->mbr, got %+v", res.Diff.PartitionTable.Type)
	}
	if res.Diff.PartitionTable.PhysicalSectorSize == nil {
		t.Fatalf("expected physical sector size diff")
	}
	if res.Diff.PartitionTable.ProtectiveMBR == nil {
		t.Fatalf("expected protective MBR diff")
	}
}

func TestCompareImages_PartitionsAddedRemovedModified_ByFSUUIDKey(t *testing.T) {
	// A has ESP + root
	a := &ImageSummary{
		File:      "a.raw",
		SizeBytes: 100,
		PartitionTable: PartitionTableSummary{
			Type:              "gpt",
			LogicalSectorSize: 512,
			Partitions: []PartitionSummary{
				{
					Index:    1,
					Name:     "ESP",
					Type:     "efi",
					StartLBA:  2048,
					EndLBA:    4095,
					SizeBytes: 1024,
					Filesystem: &FilesystemSummary{
						Type:  "vfat",
						UUID:  "UUID-ESP",
						Label: "EFI",
					},
				},
				{
					Index:    2,
					Name:     "root",
					Type:     "linux",
					StartLBA:  4096,
					EndLBA:    8191,
					SizeBytes: 2048,
					Filesystem: &FilesystemSummary{
						Type:  "ext4",
						UUID:  "UUID-ROOT",
						Label: "rootfs",
					},
				},
			},
		},
	}

	// B removes root, modifies ESP label, adds data
	b := &ImageSummary{
		File:      "b.raw",
		SizeBytes: 100,
		PartitionTable: PartitionTableSummary{
			Type:              "gpt",
			LogicalSectorSize: 512,
			Partitions: []PartitionSummary{
				{
					Index:    1,
					Name:     "ESP",
					Type:     "efi",
					StartLBA:  2048,
					EndLBA:    4095,
					SizeBytes: 1024,
					Filesystem: &FilesystemSummary{
						Type:  "vfat",
						UUID:  "UUID-ESP",
						Label: "EFI-NEW",
					},
				},
				{
					Index:    3,
					Name:     "data",
					Type:     "linux",
					StartLBA:  9000,
					EndLBA:    9999,
					SizeBytes: 4096,
					Filesystem: &FilesystemSummary{
						Type:  "ext4",
						UUID:  "UUID-DATA",
						Label: "data",
					},
				},
			},
		},
	}

	res := CompareImages(a, b)

	if res.Equal {
		t.Fatalf("expected Equal=false")
	}
	// Added: data
	if len(res.Diff.Partitions.Added) != 1 {
		t.Fatalf("expected 1 added partition, got %d", len(res.Diff.Partitions.Added))
	}
	// Removed: root
	if len(res.Diff.Partitions.Removed) != 1 {
		t.Fatalf("expected 1 removed partition, got %d", len(res.Diff.Partitions.Removed))
	}
	// Modified: ESP
	if len(res.Diff.Partitions.Modified) != 1 {
		t.Fatalf("expected 1 modified partition, got %d", len(res.Diff.Partitions.Modified))
	}
	if res.Diff.Partitions.Modified[0].Filesystem == nil || res.Diff.Partitions.Modified[0].Filesystem.Modified == nil {
		t.Fatalf("expected filesystem modified diff for ESP")
	}
	if res.Diff.Partitions.Modified[0].Filesystem.Modified.From.Label != "EFI" ||
		res.Diff.Partitions.Modified[0].Filesystem.Modified.To.Label != "EFI-NEW" {
		t.Fatalf("expected FS label change EFI->EFI-NEW, got %+v", res.Diff.Partitions.Modified[0].Filesystem.Modified)
	}
}

func TestCompareImages_EFIBinaries_ModifiedAndUKIDiff(t *testing.T) {
	a := &ImageSummary{
		File:      "a.raw",
		SizeBytes: 100,
		PartitionTable: PartitionTableSummary{
			Type:              "gpt",
			LogicalSectorSize: 512,
			Partitions: []PartitionSummary{
				{
					Index:    1,
					Name:     "ESP",
					Type:     "efi",
					StartLBA:  2048,
					EndLBA:    4095,
					SizeBytes: 1024,
					Filesystem: &FilesystemSummary{
						Type:  "vfat",
						UUID:  "UUID-ESP",
						Label: "EFI",
						EFIBinaries: []EFIBinaryEvidence{
							{
								Path:         "EFI/BOOT/BOOTX64.EFI",
								SHA256:       "aaa",
								Kind:         BootloaderUnknown,
								Arch:         "x86_64",
								Signed:       false,
								IsUKI:        true,
								KernelSHA256: "k1",
								InitrdSHA256: "i1",
							},
							{
								Path:   "EFI/BOOT/grubx64.efi",
								SHA256: "bbb",
								Kind:   BootloaderGrub,
								Arch:   "x86_64",
							},
						},
					},
				},
			},
		},
	}

	// Build b as a deep-enough copy of the partition/fs we will mutate.
	// Copy top-level
	b := &ImageSummary{
		File:      "b.raw",
		SizeBytes: a.SizeBytes,
		PartitionTable: PartitionTableSummary{
			Type:              a.PartitionTable.Type,
			LogicalSectorSize: a.PartitionTable.LogicalSectorSize,
			PhysicalSectorSize: a.PartitionTable.PhysicalSectorSize,
			ProtectiveMBR:     a.PartitionTable.ProtectiveMBR,
			Partitions:        make([]PartitionSummary, len(a.PartitionTable.Partitions)),
		},
	}

	// Copy the single partition
	b.PartitionTable.Partitions[0] = a.PartitionTable.Partitions[0]

	// Copy the filesystem struct (not pointer)
	afs := a.PartitionTable.Partitions[0].Filesystem
	if afs == nil {
		t.Fatalf("expected filesystem in test setup")
	}
	fsCopy := *afs

	// Replace EFIBinaries in b only
	fsCopy.EFIBinaries = []EFIBinaryEvidence{
		{
			Path:   "EFI/BOOT/grubx64.efi",
			SHA256: "bbb",
			Kind:   BootloaderGrub,
			Arch:   "x86_64",
		},
		{
			Path:         "EFI/BOOT/BOOTX64.EFI",
			SHA256:       "ccc",
			Kind:         BootloaderUKI,
			Arch:         "x86_64",
			Signed:       false,
			IsUKI:        true,
			KernelSHA256: "k2",
			InitrdSHA256: "i1",
		},
	}

	b.PartitionTable.Partitions[0].Filesystem = &fsCopy

	res := CompareImages(a, b)

	if res.Equal {
		t.Fatalf("expected Equal=false")
	}

	efi := res.Diff.EFIBinaries
	if len(efi.Modified) != 1 {
		t.Fatalf("expected 1 modified efi binary, got %d", len(efi.Modified))
	}
	if efi.Modified[0].Key != "EFI/BOOT/BOOTX64.EFI" {
		t.Fatalf("expected modified key BOOTX64, got %s", efi.Modified[0].Key)
	}
	if efi.Modified[0].From.Kind != BootloaderUnknown || efi.Modified[0].To.Kind != BootloaderUKI {
		t.Fatalf("expected kind unknown->uki, got %s -> %s", efi.Modified[0].From.Kind, efi.Modified[0].To.Kind)
	}
	if efi.Modified[0].UKI == nil || !efi.Modified[0].UKI.Changed {
		t.Fatalf("expected UKI diff present and changed")
	}
	if efi.Modified[0].UKI.KernelSHA256 == nil || efi.Modified[0].UKI.KernelSHA256.From != "k1" || efi.Modified[0].UKI.KernelSHA256.To != "k2" {
		t.Fatalf("expected kernel hash k1->k2, got %+v", efi.Modified[0].UKI.KernelSHA256)
	}
}

func TestCompareImages_EFIBinaries_AddedRemoved(t *testing.T) {
	a := &ImageSummary{
		File:      "a.raw",
		SizeBytes: 100,
		PartitionTable: PartitionTableSummary{
			Type:              "gpt",
			LogicalSectorSize: 512,
			Partitions: []PartitionSummary{
				{
					Index:    1,
					Name:     "ESP",
					Type:     "efi",
					Filesystem: &FilesystemSummary{
						Type: "vfat",
						UUID: "UUID-ESP",
						EFIBinaries: []EFIBinaryEvidence{
							{Path: "EFI/BOOT/grubx64.efi", SHA256: "bbb", Kind: BootloaderGrub},
						},
					},
				},
			},
		},
	}
	b := &ImageSummary{
		File:      "b.raw",
		SizeBytes: 100,
		PartitionTable: PartitionTableSummary{
			Type:              "gpt",
			LogicalSectorSize: 512,
			Partitions: []PartitionSummary{
				{
					Index:    1,
					Name:     "ESP",
					Type:     "efi",
					Filesystem: &FilesystemSummary{
						Type: "vfat",
						UUID: "UUID-ESP",
						EFIBinaries: []EFIBinaryEvidence{
							{Path: "EFI/BOOT/systemd-bootx64.efi", SHA256: "sss", Kind: BootloaderSystemdBoot},
						},
					},
				},
			},
		},
	}

	res := CompareImages(a, b)

	if len(res.Diff.EFIBinaries.Added) != 1 {
		t.Fatalf("expected 1 added efi, got %d", len(res.Diff.EFIBinaries.Added))
	}
	if len(res.Diff.EFIBinaries.Removed) != 1 {
		t.Fatalf("expected 1 removed efi, got %d", len(res.Diff.EFIBinaries.Removed))
	}
	if res.Diff.EFIBinaries.Added[0].Path != "EFI/BOOT/systemd-bootx64.efi" {
		t.Fatalf("unexpected added path: %s", res.Diff.EFIBinaries.Added[0].Path)
	}
	if res.Diff.EFIBinaries.Removed[0].Path != "EFI/BOOT/grubx64.efi" {
		t.Fatalf("unexpected removed path: %s", res.Diff.EFIBinaries.Removed[0].Path)
	}
}
