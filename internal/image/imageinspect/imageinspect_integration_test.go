package imageinspect

import (
	"os"
	"testing"
)

func TestInspectImage_Minimal(t *testing.T) {
	cases := []struct {
		name string
		img  string
	}{
		{"gpt", "gpt_disk.img"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filename := repoRootTestdataPath(t, tc.img)
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				t.Skipf("testdata not found: %s (run 'make testdata' to generate)", filename)
			}
			is := NewDiskfsInspector(false)
			got, err := is.Inspect(filename)
			if err != nil {
				t.Fatalf("inspect: %v", err)
			}

			if got.PartitionTable.Type == "" {
				t.Fatalf("PartitionTable.Type is empty")
			}
			if got.PartitionTable.LogicalSectorSize == 0 {
				t.Fatalf("PartitionTable.LogicalSectorSize is 0")
			}
			if got.PartitionTable.PhysicalSectorSize == 0 {
				t.Fatalf("PartitionTable.PhysicalSectorSize is 0")
			}

			if err := RenderSummaryText(os.Stdout, got, TextOptions{}); err != nil {
				t.Fatalf("RenderSummaryText error: %v", err)
			}
		})
	}
}

func TestInspect_Image_SanityAndInvariants(t *testing.T) {
	cases := []struct {
		name     string
		img      string
		wantType string
	}{
		{name: "gpt", img: "gpt_disk.img", wantType: "gpt"},
	}

	is := NewDiskfsInspector(false)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filename := repoRootTestdataPath(t, tc.img)
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				t.Skipf("testdata not found: %s (run 'make testdata' to generate)", filename)
			}
			got, err := is.Inspect(filename)
			if err != nil {
				t.Fatalf("inspect(%s): %v", filename, err)
			}

			require(t, got.PartitionTable.Type != "", "PartitionTable.Type empty")
			require(t, got.PartitionTable.Type == tc.wantType, "PartitionTable.Type=%q want %q", got.PartitionTable.Type, tc.wantType)

			require(t, got.PartitionTable.LogicalSectorSize > 0, "LogicalSectorSize=0")
			require(t, got.PartitionTable.PhysicalSectorSize > 0, "PhysicalSectorSize=0")

			if got.SizeBytes > 0 {
				require(t, got.SizeBytes%got.PartitionTable.LogicalSectorSize == 0,
					"SizeBytes (%d) not aligned to logical sector (%d)",
					got.SizeBytes, got.PartitionTable.LogicalSectorSize)
			}

			parts := got.PartitionTable.Partitions
			require(t, len(parts) > 0, "expected at least 1 partition")

			for i, p := range parts {
				require(t, p.SizeBytes > 0, "partition[%d] SizeBytes=0", i)
				require(t, p.EndLBA >= p.StartLBA, "partition[%d] EndLBA < StartLBA", i)

				require(t, uint64(p.SizeBytes)%uint64(got.PartitionTable.LogicalSectorSize) == 0,
					"partition[%d] size not sector-aligned: size=%d sector=%d",
					i, p.SizeBytes, got.PartitionTable.LogicalSectorSize)
			}

			for i := 1; i < len(parts); i++ {
				prev := parts[i-1]
				cur := parts[i]
				require(t, cur.StartLBA > prev.EndLBA,
					"partitions overlap or not strictly increasing: [%d] end=%d, [%d] start=%d",
					i-1, prev.EndLBA, i, cur.StartLBA)
			}

			if got.PartitionTable.Type == "gpt" {
				require(t, got.PartitionTable.ProtectiveMBR, "expected protective MBR for GPT, got false")
			}
		})
	}
}
