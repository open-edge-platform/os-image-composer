package debutils_test

import (
	"testing"

	"github.com/open-edge-platform/image-composer/internal/debutils"
	"github.com/open-edge-platform/image-composer/internal/resolvertest"
)

func TestDEBResolver(t *testing.T) {
	resolvertest.RunResolverTestsFunc(
		t,
		"debutils",
		debutils.ResolvePackageInfos, // directly passing your function
	)
}
