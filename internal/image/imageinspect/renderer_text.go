package imageinspect

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// PrintSummary prints a human-readable summary of the image inspection to the given writer.
func PrintSummary(w io.Writer, summary *ImageSummary) {

	if summary == nil {
		log.Errorf("PrintSummary: summary is nil")
		return
	}

	// Header
	fmt.Fprintln(w, "OS Image Summary")
	fmt.Fprintln(w, "================")
	fmt.Fprintf(w, "Image:\t%s\n", summary.File)
	fmt.Fprintf(w, "Size:\t%s (%d bytes)\n", humanBytes(summary.SizeBytes), summary.SizeBytes)

	// Partition table section
	pt := summary.PartitionTable
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Partition Table")
	fmt.Fprintln(w, "---------------")
	fmt.Fprintf(w, "Type:\t%s\n", strings.ToUpper(emptyIfWhitespace(pt.Type)))
	if pt.LogicalSectorSize > 0 {
		fmt.Fprintf(w, "Logical sector size:\t%d bytes\n", pt.LogicalSectorSize)
	}
	if pt.PhysicalSectorSize > 0 {
		fmt.Fprintf(w, "Physical sector size:\t%d bytes\n", pt.PhysicalSectorSize)
	}
	if strings.EqualFold(pt.Type, "gpt") {
		fmt.Fprintf(w, "Protective MBR:\t%t\n", pt.ProtectiveMBR)
	}

	// Partitions table
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Partitions")
	fmt.Fprintln(w, "----------")

	if len(pt.Partitions) == 0 {
		fmt.Fprintln(w, "(none)")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "IDX\tNAME\tROLE\tPTYPE\tPTYPE_NAME\tSTART(LBA)\tEND(LBA)\tSIZE\tFLAGS\tFS\tLABEL/ID")
	for _, p := range pt.Partitions {
		fsType := "-"
		fsLabelOrID := "-"

		if p.Filesystem != nil {
			if s := strings.TrimSpace(p.Filesystem.Type); s != "" {
				fsType = s
			}
			// Show FAT type inline (vfat(FAT16/FAT32))
			if strings.EqualFold(fsType, "vfat") && strings.TrimSpace(p.Filesystem.FATType) != "" {
				fsType = fmt.Sprintf("vfat(%s)", strings.TrimSpace(p.Filesystem.FATType))
			}

			lbl := strings.TrimSpace(p.Filesystem.Label)
			id := strings.TrimSpace(p.Filesystem.UUID)
			switch {
			case lbl != "" && id != "":
				fsLabelOrID = fmt.Sprintf("%s (%s)", lbl, id)
			case lbl != "":
				fsLabelOrID = lbl
			case id != "":
				fsLabelOrID = id
			}
		}

		ptypeName := "-"
		if s := partitionTypeName(pt.Type, p.Type); s != "" {
			ptypeName = s
		}
		role := partitionRole(p)

		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\t%s\n",
			p.Index,
			emptyIfWhitespace(p.Name),
			role,
			emptyIfWhitespace(p.Type),
			ptypeName,
			p.StartLBA,
			p.EndLBA,
			humanBytes(int64(p.SizeBytes)),
			emptyIfWhitespace(p.Flags),
			fsType,
			fsLabelOrID,
		)
	}
	_ = tw.Flush()

	// Detailed per-partition filesystem blocks
	for _, p := range pt.Partitions {
		if p.Filesystem == nil {
			continue
		}
		fs := p.Filesystem
		if isFilesystemEmpty(fs) {
			continue
		}

		fmt.Fprintln(w)
		fmt.Fprintf(w, "Partition %d filesystem details\n", p.Index)
		fmt.Fprintln(w, "------------------------------")

		// Key/value lines
		kv := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
		fmt.Fprintf(kv, "FS type:\t%s\n", emptyIfWhitespace(fs.Type))
		if strings.TrimSpace(fs.Label) != "" {
			fmt.Fprintf(kv, "Label:\t%s\n", fs.Label)
		}
		if strings.TrimSpace(fs.UUID) != "" {
			fmt.Fprintf(kv, "UUID/ID:\t%s\n", fs.UUID)
		}
		if fs.BlockSize > 0 {
			fmt.Fprintf(kv, "Block size:\t%d\n", fs.BlockSize)
		}
		if len(fs.Features) > 0 {
			fmt.Fprintf(kv, "Features:\t%s\n", strings.Join(fs.Features, ", "))
		}

		// VFAT-specific
		if isVFATLike(fs.Type) {
			if fs.FATType != "" {
				fmt.Fprintf(kv, "FAT type:\t%s\n", fs.FATType)
			}
			if fs.BytesPerSector != 0 {
				fmt.Fprintf(kv, "Bytes/sector:\t%d\n", fs.BytesPerSector)
			}
			if fs.SectorsPerCluster != 0 {
				fmt.Fprintf(kv, "Sectors/cluster:\t%d\n", fs.SectorsPerCluster)
			}
			if fs.ClusterCount != 0 {
				fmt.Fprintf(kv, "Clusters:\t%d\n", fs.ClusterCount)
			}
			if fs.BytesPerSector != 0 && fs.SectorsPerCluster != 0 {
				clusterSize := uint64(fs.BytesPerSector) * uint64(fs.SectorsPerCluster)
				fmt.Fprintf(kv, "Cluster size:\t%s (%d bytes)\n", humanBytes(int64(clusterSize)), clusterSize)
			}
		}

		// Print shim/UKI flags only if true (avoid noise)
		if fs.HasShim {
			fmt.Fprintf(kv, "Shim detected:\t%t\n", fs.HasShim)
		}
		if fs.HasUKI {
			fmt.Fprintf(kv, "UKI detected:\t%t\n", fs.HasUKI)
		}
		_ = kv.Flush()

		// EFI artifacts table (preferred)
		if isVFATLike(fs.Type) && len(fs.EFIBinaries) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintf(w, "EFI artifacts:\t%d\n", len(fs.EFIBinaries))

			tw2 := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw2, "KIND\tSIGNED\tARCH\tPATH\tSIZE\tSHA256\tKERNEL\tINITRD")

			// Sort by path for stable output
			arts := append([]EFIBinaryEvidence(nil), fs.EFIBinaries...)
			sort.Slice(arts, func(i, j int) bool { return arts[i].Path < arts[j].Path })

			for _, a := range arts {
				kernel := "-"
				initrd := "-"
				if a.KernelSHA256 != "" {
					kernel = shortHash(a.KernelSHA256)
				}
				if a.InitrdSHA256 != "" {
					initrd = shortHash(a.InitrdSHA256)
				}
				fmt.Fprintf(tw2, "%s\t%t\t%s\t%s\t%s\t%s\t%s\t%s\n",
					emptyOr(string(a.Kind), "unknown"),
					a.Signed,
					emptyOr(a.Arch, "-"),
					emptyIfWhitespace(a.Path),
					humanBytes(a.Size),
					shortHash(a.SHA256),
					kernel,
					initrd,
				)
			}
			_ = tw2.Flush()

			// Print a focused UKI block for the first UKI found (helps a ton for humans)
			if uki, ok := firstUKI(arts); ok {
				fmt.Fprintln(w)
				fmt.Fprintln(w, "UKI details")
				fmt.Fprintln(w, "-----------")
				kv2 := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
				fmt.Fprintf(kv2, "Path:\t%s\n", uki.Path)
				if uki.Arch != "" {
					fmt.Fprintf(kv2, "Architecture:\t%s\n", uki.Arch)
				}
				if uki.Uname != "" {
					fmt.Fprintf(kv2, "EFI uname:\t%s\n", uki.Uname)
				}
				if uki.Cmdline != "" {
					fmt.Fprintf(kv2, "EFI cmdline:\t%s\n", uki.Cmdline)
				}
				if uki.KernelSHA256 != "" {
					fmt.Fprintf(kv2, "Kernel SHA256:\t%s\n", uki.KernelSHA256)
				}
				if uki.InitrdSHA256 != "" {
					fmt.Fprintf(kv2, "Initrd SHA256:\t%s\n", uki.InitrdSHA256)
				}
				if len(uki.OSReleaseSorted) > 0 {
					_ = kv2.Flush()

					printOSReleaseKV(w, "EFI OS release:", uki.OSReleaseSorted)

				} else if uki.OSReleaseRaw != "" {
					// fallback: raw only if we couldn't parse anything
					_ = kv2.Flush()
					fmt.Fprintln(w)
					fmt.Fprintln(w, "EFI OS release:")
					fmt.Fprintln(w, uki.OSReleaseRaw)

				} else {
					_ = kv2.Flush()
				}
			}
		}

		// Squashfs-specific
		if strings.EqualFold(fs.Type, "squashfs") {
			fmt.Fprintln(w)
			kv3 := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
			if fs.Compression != "" {
				fmt.Fprintf(kv3, "Compression:\t%s\n", fs.Compression)
			}
			if fs.Version != "" {
				fmt.Fprintf(kv3, "Version:\t%s\n", fs.Version)
			}
			if len(fs.FsFlags) > 0 {
				fmt.Fprintf(kv3, "Flags:\t%s\n", strings.Join(fs.FsFlags, ", "))
			}
			_ = kv3.Flush()
		}

		// Notes
		if len(fs.Notes) > 0 {
			fmt.Fprintln(w, "Notes:")
			for _, note := range fs.Notes {
				fmt.Fprintf(w, "  - %s\n", note)
			}
		}
	}

	fmt.Fprintln(w)
}

func humanBytes(n int64) string {
	if n < 0 {
		return fmt.Sprintf("%d B", n)
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func partitionTypeName(ptType, pType string) string {
	ptType = strings.ToLower(strings.TrimSpace(ptType))
	switch ptType {
	case "gpt":
		return gptTypeName(pType)
	case "mbr":
		return mbrTypeName(pType)
	default:
		return ""
	}
}

func gptTypeName(guid string) string {
	switch strings.ToUpper(strings.TrimSpace(guid)) {
	case "C12A7328-F81F-11D2-BA4B-00A0C93EC93B":
		return "EFI System Partition"
	case "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709":
		return "Linux root (x86-64)"
	case "0FC63DAF-8483-4772-8E79-3D69D8477DE4":
		return "Linux filesystem"
	case "21686148-6449-6E6F-744E-656564454649":
		return "BIOS boot partition"
	// Add more as you run into them (BIOS boot, swap, LVM, etc.)
	default:
		return ""
	}
}

func mbrTypeName(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "0x07":
		return "HPFS/NTFS/exFAT"
	case "0x0b":
		return "W95 FAT32"
	case "0x0c":
		return "W95 FAT32 (LBA)"
	case "0x0e":
		return "W95 FAT16 (LBA)"
	case "0x82":
		return "Linux swap"
	case "0x83":
		return "Linux filesystem"
	case "0x8e":
		return "Linux LVM"
	case "0xaf":
		return "Apple HFS/HFS+"
	default:
		return ""
	}
}
func partitionRole(p PartitionSummary) string {

	// Light heuristic: prefer GPT type, then FS type.
	if name := gptTypeName(p.Type); name != "" {
		if name == "EFI System Partition" {
			return "ESP"
		}
		if strings.HasPrefix(name, "Linux root") {
			return "ROOT"
		}
		if name == "Linux filesystem" {
			// you can specialize based on name
			if strings.Contains(strings.ToLower(p.Name), "userdata") {
				return "DATA"
			}
			return "FS"
		}
		return name
	}

	if p.Filesystem != nil {
		switch strings.ToLower(p.Filesystem.Type) {
		case "vfat":
			return "ESP?"
		case "ext4":
			return "FS"
		case "squashfs":
			return "SQUASHFS"
		}
	}
	return "-"
}

func isFilesystemEmpty(fs *FilesystemSummary) bool {
	if fs == nil {
		return true
	}
	// If these are all empty/zero, there’s nothing worth printing.
	return strings.TrimSpace(fs.Type) == "" &&
		strings.TrimSpace(fs.Label) == "" &&
		strings.TrimSpace(fs.UUID) == "" &&
		fs.BlockSize == 0 &&
		len(fs.Features) == 0 &&
		len(fs.Notes) == 0 &&
		fs.FATType == "" &&
		fs.BytesPerSector == 0 &&
		fs.SectorsPerCluster == 0 &&
		fs.Compression == "" &&
		fs.Version == "" &&
		len(fs.FsFlags) == 0 &&
		!fs.HasShim &&
		!fs.HasUKI
}

func emptyIfWhitespace(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return strings.TrimSpace(s)
}

func emptyOr(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}
func shortHash(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

func firstUKI(arts []EFIBinaryEvidence) (EFIBinaryEvidence, bool) {
	for _, a := range arts {
		if a.IsUKI || a.Kind == BootloaderUKI {
			return a, true
		}
	}
	return EFIBinaryEvidence{}, false
}

func printOSReleaseKV(w io.Writer, title string, kvs []KeyValue) {
	if len(kvs) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, title)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, kv := range kvs {
		fmt.Fprintf(tw, "%s:\t%q\n", kv.Key, kv.Value)
	}
	_ = tw.Flush()
}
