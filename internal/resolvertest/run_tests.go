package resolvertest

import (
	"reflect"
	"sort"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/provider"
)

// Resolver is interface both rpmutil & debutil satisfy.
type Resolver interface {
	Resolve(
		requested []provider.PackageInfo,
		all []provider.PackageInfo,
	) ([]provider.PackageInfo, error)
}

// helper to extract and sort names from PackageInfo slice
func names(ps []provider.PackageInfo) []string {
	var outs []string
	for _, p := range ps {
		outs = append(outs, p.Name)
	}
	sort.Strings(outs)
	return outs
}

var TestCases = []struct {
	Name      string
	Requested []provider.PackageInfo
	All       []provider.PackageInfo
	Want      []string
	WantErr   bool
}{
	{
		Name: "SimpleChain",
		All: []provider.PackageInfo{
			{Name: "C", Provides: []string{"C"}, Requires: []string{}},
			{Name: "B", Provides: []string{"B"}, Requires: []string{"C"}},
			{Name: "A", Provides: []string{"A"}, Requires: []string{"B"}},
		},
		Requested: []provider.PackageInfo{
			{Name: "A", Provides: []string{"A"}, Requires: []string{"B"}},
		},
		Want:    []string{"A", "B", "C"},
		WantErr: false,
	},
	{
		Name: "MultipleProviders",
		All: []provider.PackageInfo{
			{Name: "Y", Provides: []string{"Y"}, Requires: []string{}},
			{Name: "P1", Provides: []string{"X"}, Requires: []string{}},
			{Name: "P2", Provides: []string{"X"}, Requires: []string{"Y"}},
			{Name: "A", Provides: []string{"A"}, Requires: []string{"X"}},
		},
		Requested: []provider.PackageInfo{
			{Name: "A", Provides: []string{"A"}, Requires: []string{"X"}},
		},
		Want:    []string{"A", "P2", "Y"},
		WantErr: false,
	},
	{
		Name: "NoDependencies",
		All: []provider.PackageInfo{
			{Name: "X", Provides: []string{"X"}, Requires: []string{}},
		},
		Requested: []provider.PackageInfo{
			{Name: "X", Provides: []string{"A"}, Requires: []string{"X"}},
		},
		Want:    []string{"X"},
		WantErr: false,
	},
	{
		Name: "MissingRequested",
		All: []provider.PackageInfo{
			{Name: "A", Provides: []string{"A"}, Requires: []string{}},
		},
		Requested: []provider.PackageInfo{
			{Name: "B", Provides: []string{"B"}, Requires: []string{""}},
		},
		Want:    []string{},
		WantErr: true,
	},
}

// RunResolverTestsFunc drives a bare function through your table.
func RunResolverTestsFunc(
	t *testing.T,
	prefix string,
	resolverFunc func(requested, all []provider.PackageInfo) ([]provider.PackageInfo, error),
) {

	t.Helper()
	for _, tc := range TestCases {
		t.Run(prefix+"/"+tc.Name, func(t *testing.T) {
			got, err := resolverFunc(tc.Requested, tc.All)
			if (err != nil) != tc.WantErr {
				t.Fatalf("err = %v, wantErr? %v", err, tc.WantErr)
			}

			if !tc.WantErr && !reflect.DeepEqual(names(got), tc.Want) {
				t.Errorf("ResolvePackageInfos [%v] = %v; want %v", tc.Name, names(got), tc.Want)
			}
		})
	}
}
