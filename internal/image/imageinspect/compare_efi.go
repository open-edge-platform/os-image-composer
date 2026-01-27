package imageinspect

import "sort"

func flattenEFIBinaries(pt PartitionTableSummary) []EFIBinaryEvidence {
	var out []EFIBinaryEvidence
	for _, p := range pt.Partitions {
		out = append(out, flattenEFIBinariesFromPartition(p)...)
	}
	// Ensure deterministic order
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func flattenEFIBinariesFromPartition(p PartitionSummary) []EFIBinaryEvidence {
	if p.Filesystem == nil {
		return nil
	}
	// Evidence only exists if the inspector populated it
	if len(p.Filesystem.EFIBinaries) == 0 {
		return nil
	}
	// Copy to avoid accidental sharing/mutation
	out := append([]EFIBinaryEvidence(nil), p.Filesystem.EFIBinaries...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func compareEFIBinaries(from, to []EFIBinaryEvidence) EFIBinaryDiff {
	out := EFIBinaryDiff{}

	fm := make(map[string]EFIBinaryEvidence, len(from))
	tm := make(map[string]EFIBinaryEvidence, len(to))

	for _, e := range from {
		fm[e.Path] = e
	}
	for _, e := range to {
		tm[e.Path] = e
	}

	keys := make([]string, 0, len(fm)+len(tm))
	seen := map[string]struct{}{}
	for k := range fm {
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for k := range tm {
		if _, ok := seen[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, k := range keys {
		f, fok := fm[k]
		t, tok := tm[k]

		switch {
		case fok && !tok:
			out.Removed = append(out.Removed, f)
		case !fok && tok:
			out.Added = append(out.Added, t)
		case fok && tok:
			if efiEvidenceEqual(f, t) {
				continue
			}
			mod := ModifiedEFIBinaryEvidence{
				Key:  k,
				From: f,
				To:   t,
			}
			mod.Changes = appendEFIBinaryFieldChanges(nil, f, t)
			mod.UKI = buildUKIDiffIfRelevant(f, t)
			out.Modified = append(out.Modified, mod)
		}
	}

	return out
}

func appendEFIBinaryFieldChanges(dst []FieldChange, a, b EFIBinaryEvidence) []FieldChange {
	add := func(field string, from, to any) {
		dst = append(dst, FieldChange{Field: field, From: from, To: to})
	}
	if a.SHA256 != b.SHA256 {
		add("sha256", a.SHA256, b.SHA256)
	}
	if a.Size != b.Size {
		add("size", a.Size, b.Size)
	}
	if a.Arch != b.Arch {
		add("arch", a.Arch, b.Arch)
	}
	if a.Kind != b.Kind {
		add("kind", a.Kind, b.Kind)
	}
	if a.Signed != b.Signed {
		add("signed", a.Signed, b.Signed)
	}
	if a.SignatureSize != b.SignatureSize {
		add("signatureSize", a.SignatureSize, b.SignatureSize)
	}
	if a.HasSBAT != b.HasSBAT {
		add("hasSbat", a.HasSBAT, b.HasSBAT)
	}
	if a.IsUKI != b.IsUKI {
		add("isUki", a.IsUKI, b.IsUKI)
	}

	// UKI payload hashes (high value)
	if a.KernelSHA256 != b.KernelSHA256 {
		add("kernelSha256", a.KernelSHA256, b.KernelSHA256)
	}
	if a.InitrdSHA256 != b.InitrdSHA256 {
		add("initrdSha256", a.InitrdSHA256, b.InitrdSHA256)
	}
	if a.CmdlineSHA256 != b.CmdlineSHA256 {
		add("cmdlineSha256", a.CmdlineSHA256, b.CmdlineSHA256)
	}
	if a.OSRelSHA256 != b.OSRelSHA256 {
		add("osrelSha256", a.OSRelSHA256, b.OSRelSHA256)
	}
	if a.UnameSHA256 != b.UnameSHA256 {
		add("unameSha256", a.UnameSHA256, b.UnameSHA256)
	}

	return dst
}

func buildUKIDiffIfRelevant(a, b EFIBinaryEvidence) *UKIDiff {
	// Only include if either side is a UKI OR has UKI payload hashes
	if !(a.IsUKI || b.IsUKI || a.Kind == BootloaderUKI || b.Kind == BootloaderUKI) {
		// Still could be useful if hashes exist but IsUKI not set.
		if a.KernelSHA256 == "" && b.KernelSHA256 == "" &&
			a.InitrdSHA256 == "" && b.InitrdSHA256 == "" &&
			a.CmdlineSHA256 == "" && b.CmdlineSHA256 == "" &&
			a.OSRelSHA256 == "" && b.OSRelSHA256 == "" &&
			a.UnameSHA256 == "" && b.UnameSHA256 == "" &&
			len(a.SectionSHA256) == 0 && len(b.SectionSHA256) == 0 {
			return nil
		}
	}

	d := &UKIDiff{}
	if a.KernelSHA256 != b.KernelSHA256 {
		d.KernelSHA256 = &ValueDiff[string]{From: a.KernelSHA256, To: b.KernelSHA256}
		d.Changed = true
	}
	if a.InitrdSHA256 != b.InitrdSHA256 {
		d.InitrdSHA256 = &ValueDiff[string]{From: a.InitrdSHA256, To: b.InitrdSHA256}
		d.Changed = true
	}
	if a.CmdlineSHA256 != b.CmdlineSHA256 {
		d.CmdlineSHA256 = &ValueDiff[string]{From: a.CmdlineSHA256, To: b.CmdlineSHA256}
		d.Changed = true
	}
	if a.OSRelSHA256 != b.OSRelSHA256 {
		d.OSRelSHA256 = &ValueDiff[string]{From: a.OSRelSHA256, To: b.OSRelSHA256}
		d.Changed = true
	}
	if a.UnameSHA256 != b.UnameSHA256 {
		d.UnameSHA256 = &ValueDiff[string]{From: a.UnameSHA256, To: b.UnameSHA256}
		d.Changed = true
	}

	// Map diff for SectionSHA256
	sd := diffStringMap(a.SectionSHA256, b.SectionSHA256)
	if len(sd.Added) > 0 || len(sd.Removed) > 0 || len(sd.Modified) > 0 {
		d.SectionSHA256 = sd
		d.Changed = true
	}

	if !d.Changed {
		return nil
	}
	return d
}

func diffStringMap(a, b map[string]string) SectionMapDiff {
	out := SectionMapDiff{
		Added:    map[string]string{},
		Removed:  map[string]string{},
		Modified: map[string]ValueDiff[string]{},
	}

	// nil-safe
	if a == nil {
		a = map[string]string{}
	}
	if b == nil {
		b = map[string]string{}
	}

	seen := map[string]struct{}{}
	for k, av := range a {
		seen[k] = struct{}{}
		if bv, ok := b[k]; !ok {
			out.Removed[k] = av
		} else if bv != av {
			out.Modified[k] = ValueDiff[string]{From: av, To: bv}
		}
	}
	for k, bv := range b {
		if _, ok := seen[k]; ok {
			continue
		}
		out.Added[k] = bv
	}

	// Normalize empties to nil for omitempty friendliness
	if len(out.Added) == 0 {
		out.Added = nil
	}
	if len(out.Removed) == 0 {
		out.Removed = nil
	}
	if len(out.Modified) == 0 {
		out.Modified = nil
	}

	return out
}
