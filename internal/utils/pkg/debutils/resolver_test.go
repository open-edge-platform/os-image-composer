package debutils_test

import (
	"testing"

	"github.com/open-edge-platform/image-composer/internal/utils/pkg/debutils"
	"github.com/open-edge-platform/image-composer/internal/utils/pkg/resolvertest"
)

func TestDEBResolver(t *testing.T) {
	resolvertest.RunResolverTestsFunc(
		t,
		"debutils",
		debutils.ResolvePackageInfos, // directly passing your function
	)
}
