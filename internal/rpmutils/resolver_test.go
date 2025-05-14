package rpmutils_test

import (
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/rpmutils"
	"reflect"
	"sort"
	"testing"
)

// helper to extract and sort names from PackageInfo slice
func names(ps []provider.PackageInfo) []string {
	var outs []string
	for _, p := range ps {
		outs = append(outs, p.Name)
	}
	sort.Strings(outs)
	return outs
}

func TestResolvePackageInfos_SimpleChain(t *testing.T) {
	// A -> requires B -> requires C
	all := []provider.PackageInfo{
		{Name: "C", Provides: []string{"C"}, Requires: []string{}},
		{Name: "B", Provides: []string{"B"}, Requires: []string{"C"}},
		{Name: "A", Provides: []string{"A"}, Requires: []string{"B"}},
	}
	requested := []provider.PackageInfo{{Name: "A", Provides: []string{"A"}, Requires: []string{"B"}}}

	got, err := rpmutils.ResolvePackageInfos(requested, all)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"A", "B", "C"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("ResolvePackageInfos simple chain = %v; want %v", names(got), want)
	}
}

func TestResolvePackageInfos_MultipleProviders(t *testing.T) {
	// A requires X, X provided by P1 and P2, P2 requires Y
	all := []provider.PackageInfo{
		{Name: "Y", Provides: []string{"Y"}, Requires: []string{}},
		{Name: "P1", Provides: []string{"X"}, Requires: []string{}},
		{Name: "P2", Provides: []string{"X"}, Requires: []string{"Y"}},
		{Name: "A", Provides: []string{"A"}, Requires: []string{"X"}},
	}
	requested := []provider.PackageInfo{{Name: "A", Provides: []string{"A"}, Requires: []string{"X"}}}

	got, err := rpmutils.ResolvePackageInfos(requested, all)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"A", "P1", "P2", "Y"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("ResolvePackageInfos multi providers = %v; want %v", names(got), want)
	}
}

func TestResolvePackageInfos_NoDeps(t *testing.T) {
	all := []provider.PackageInfo{{Name: "X", Provides: []string{"X"}, Requires: []string{}}}
	requested := []provider.PackageInfo{{Name: "X", Provides: []string{"X"}, Requires: []string{}}}

	got, err := rpmutils.ResolvePackageInfos(requested, all)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"X"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("ResolvePackageInfos no deps = %v; want %v", names(got), want)
	}
}

func TestResolvePackageInfos_MissingRequested(t *testing.T) {
	all := []provider.PackageInfo{{Name: "A", Provides: []string{"A"}, Requires: []string{}}}
	requested := []provider.PackageInfo{{Name: "B", Provides: []string{"B"}, Requires: []string{}}}

	_, err := rpmutils.ResolvePackageInfos(requested, all)
	if err == nil {
		t.Fatalf("expected error for missing requested, got none")
	}
}
