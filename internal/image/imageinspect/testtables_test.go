package imageinspect

import (
	"bytes"
	"io"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/diskfs/go-diskfs/partition/mbr"
)

type fakeDiskAccessor struct {
	pt    partition.Table
	ptErr error

	fsByPart    map[int]filesystem.FileSystem
	fsErrByPart map[int]error
	fsErrAny    error

	calls struct {
		getPT int
		getFS []int
	}
}


func (f *fakeDiskAccessor) GetPartitionTable() (partition.Table, error) {
	f.calls.getPT++
	if f.ptErr != nil {
		return nil, f.ptErr
	}
	return f.pt, nil
}

func (f *fakeDiskAccessor) GetFilesystem(partitionNumber int) (filesystem.FileSystem, error) {
	f.calls.getFS = append(f.calls.getFS, partitionNumber)

	if f.fsErrAny != nil {
		return nil, f.fsErrAny
	}
	if err, ok := f.fsErrByPart[partitionNumber]; ok && err != nil {
		return nil, err
	}
	if fs, ok := f.fsByPart[partitionNumber]; ok {
		return fs, nil
	}
	return nil, nil
}

func minimalGPTWithOnePartition() *gpt.Table {
	return &gpt.Table{
		PhysicalSectorSize: 4096,
		LogicalSectorSize:  512,
		ProtectiveMBR:      true,
		Partitions: []*gpt.Partition{
			{
				Start: 2048,
				End:   4095,
				Name:  "ESP",
			},
		},
	}
}

func minimalMBRWithOnePartition() *mbr.Table {
	return &mbr.Table{
		PhysicalSectorSize: 4096,
		LogicalSectorSize:  512,
		Partitions: []*mbr.Partition{
			{
				Type:  0x83,
				Start: 2048,
				Size:  2048,
			},
		},
	}
}

func repoRootTestdataPath(t *testing.T, filename string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot get caller info")
	}
	pkgDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(pkgDir, "..", "..", ".."))
	return filepath.Join(repoRoot, "testdata", filename)
}

func tinyReaderAt(n int) io.ReaderAt {
	return bytes.NewReader(make([]byte, n))
}

func require(t *testing.T, cond bool, msg string, args ...any) {
	t.Helper()
	if !cond {
		t.Fatalf(msg, args...)
	}
}
