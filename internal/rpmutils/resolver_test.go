package rpmutils_test

import (
	"testing"

	"github.com/open-edge-platform/image-composer/internal/resolvertest"
	"github.com/open-edge-platform/image-composer/internal/rpmutils"
)

func TestRPMResolver(t *testing.T) {
	resolvertest.RunResolverTestsFunc(
		t,
		"rpmutils",
		rpmutils.ResolvePackageInfos, // directly passing your function
	)
}
