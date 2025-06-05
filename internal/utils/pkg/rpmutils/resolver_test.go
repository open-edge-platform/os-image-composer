package rpmutils_test

import (
	"testing"

	"github.com/open-edge-platform/image-composer/internal/utils/pkg/resolvertest"
	"github.com/open-edge-platform/image-composer/internal/utils/pkg/rpmutils"
)

func TestRPMResolver(t *testing.T) {
	resolvertest.RunResolverTestsFunc(
		t,
		"rpmutils",
		rpmutils.ResolvePackageInfos, // directly passing your function
	)
}
