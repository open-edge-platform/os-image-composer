package imageinspect

import "sort"

func normalizeCompareResult(r *ImageCompareResult) {
	// Partitions
	sort.Slice(r.Diff.Partitions.Added, func(i, j int) bool {
		return partitionKey(r.Diff.Partitions.Added[i]) < partitionKey(r.Diff.Partitions.Added[j])
	})
	sort.Slice(r.Diff.Partitions.Removed, func(i, j int) bool {
		return partitionKey(r.Diff.Partitions.Removed[i]) < partitionKey(r.Diff.Partitions.Removed[j])
	})
	sort.Slice(r.Diff.Partitions.Modified, func(i, j int) bool {
		return r.Diff.Partitions.Modified[i].Key < r.Diff.Partitions.Modified[j].Key
	})

	// EFI
	sort.Slice(r.Diff.EFIBinaries.Added, func(i, j int) bool { return r.Diff.EFIBinaries.Added[i].Path < r.Diff.EFIBinaries.Added[j].Path })
	sort.Slice(r.Diff.EFIBinaries.Removed, func(i, j int) bool { return r.Diff.EFIBinaries.Removed[i].Path < r.Diff.EFIBinaries.Removed[j].Path })
	sort.Slice(r.Diff.EFIBinaries.Modified, func(i, j int) bool { return r.Diff.EFIBinaries.Modified[i].Key < r.Diff.EFIBinaries.Modified[j].Key })

	// sort per-partition EFI diffs if present
	for i := range r.Diff.Partitions.Modified {
		m := &r.Diff.Partitions.Modified[i]
		if m.EFIBinaries == nil {
			continue
		}
		sort.Slice(m.EFIBinaries.Added, func(i, j int) bool { return m.EFIBinaries.Added[i].Path < m.EFIBinaries.Added[j].Path })
		sort.Slice(m.EFIBinaries.Removed, func(i, j int) bool { return m.EFIBinaries.Removed[i].Path < m.EFIBinaries.Removed[j].Path })
		sort.Slice(m.EFIBinaries.Modified, func(i, j int) bool { return m.EFIBinaries.Modified[i].Key < m.EFIBinaries.Modified[j].Key })
	}
}
