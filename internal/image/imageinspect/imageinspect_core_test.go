package imageinspect

import (
	"errors"
	"strings"
	"testing"
)

func TestInspectCore_Propagates_GetPartitionTable_Error(t *testing.T) {
	d := &DiskfsInspector{}
	img := tinyReaderAt(4096)

	want := errors.New("pt boom")
	disk := &fakeDiskAccessor{ptErr: want}

	_, err := d.inspectCore(img, disk, 512, "ignored", 1<<20)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("err=%v want wrapping %v", err, want)
	}
	if disk.calls.getPT != 1 {
		t.Fatalf("GetPartitionTable calls=%d want 1", disk.calls.getPT)
	}
}

func TestInspectCore_GPT_Table_SetsTypeAndBasics(t *testing.T) {
	d := &DiskfsInspector{}
	img := tinyReaderAt(4096)

	disk := &fakeDiskAccessor{pt: minimalGPTWithOnePartition()}

	got, err := d.inspectCore(img, disk, 512, "ignored", 8<<20)
	if err != nil {
		t.Fatalf("inspectCore: %v", err)
	}
	if got.PartitionTable.Type != "gpt" {
		t.Fatalf("PartitionTable.Type=%q want gpt", got.PartitionTable.Type)
	}
	require(t, len(got.PartitionTable.Partitions) > 0, "expected at least 1 partition")
}

func TestInspectCore_MBR_Table_SetsTypeAndBasics(t *testing.T) {
	d := &DiskfsInspector{}
	img := tinyReaderAt(4096)

	disk := &fakeDiskAccessor{pt: minimalMBRWithOnePartition()}

	got, err := d.inspectCore(img, disk, 512, "ignored", 8<<20)
	if err != nil {
		t.Fatalf("inspectCore: %v", err)
	}
	if got.PartitionTable.Type != "mbr" {
		t.Fatalf("PartitionTable.Type=%q want mbr", got.PartitionTable.Type)
	}
	require(t, len(got.PartitionTable.Partitions) > 0, "expected at least 1 partition")
}

func TestInspectCore_GetFilesystem_Error_IsRecordedAsNote(t *testing.T) {
	d := &DiskfsInspector{}
	img := tinyReaderAt(4096)

	want := errors.New("fs boom")
	disk := &fakeDiskAccessor{
		pt:       minimalGPTWithOnePartition(),
		fsErrAny: want, // any filesystem open fails
	}

	got, err := d.inspectCore(img, disk, 512, "ignored", 8<<20)
	if err != nil {
		t.Fatalf("inspectCore should not fail on GetFilesystem error; got: %v", err)
	}

	require(t, len(disk.calls.getFS) > 0, "expected GetFilesystem to be called at least once")

	parts := got.PartitionTable.Partitions
	require(t, len(parts) > 0, "expected partitions")
	require(t, parts[0].Filesystem != nil, "expected Filesystem to be non-nil")

	notes := strings.Join(parts[0].Filesystem.Notes, "\n")
	require(t, strings.Contains(notes, "diskfs GetFilesystem("), "expected GetFilesystem note; got notes:\n%s", notes)
	require(t, strings.Contains(notes, "fs boom"), "expected error text in notes; got notes:\n%s", notes)
}

func TestSummarizePartitionTable_LogicalBlockSizeAffectsSizeBytes(t *testing.T) {
	pt := minimalGPTWithOnePartition()

	a, err := summarizePartitionTable(pt, 512)
	if err != nil {
		t.Fatal(err)
	}
	b, err := summarizePartitionTable(pt, 4096)
	if err != nil {
		t.Fatal(err)
	}

	if a.Partitions[0].SizeBytes*8 != b.Partitions[0].SizeBytes {
		t.Fatalf("expected 4096-byte blocks to produce 8x size: a=%d b=%d", a.Partitions[0].SizeBytes, b.Partitions[0].SizeBytes)
	}
}
