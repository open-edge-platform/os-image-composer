package imageinspect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalSPDXSHA256_StableAcrossOrder(t *testing.T) {
	first := []byte(`{
  "packages": [
    {
      "name": "zlib",
      "versionInfo": "1.2.13",
      "downloadLocation": "https://example.com/zlib.rpm",
      "checksum": [
        {"algorithm": "SHA1", "checksumValue": "bbb"},
        {"algorithm": "SHA256", "checksumValue": "aaa"}
      ]
    },
    {
      "name": "acl",
      "versionInfo": "2.3.1",
      "downloadLocation": "https://example.com/acl.rpm",
      "checksum": [
        {"algorithm": "SHA256", "checksumValue": "ccc"}
      ]
    }
  ]
}`)

	second := []byte(`{
  "packages": [
    {
      "name": "acl",
      "versionInfo": "2.3.1",
      "downloadLocation": "https://example.com/acl.rpm",
      "checksum": [
        {"algorithm": "SHA256", "checksumValue": "ccc"}
      ]
    },
    {
      "name": "zlib",
      "versionInfo": "1.2.13",
      "downloadLocation": "https://example.com/zlib.rpm",
      "checksum": [
        {"algorithm": "SHA256", "checksumValue": "aaa"},
        {"algorithm": "SHA1", "checksumValue": "bbb"}
      ]
    }
  ]
}`)

	h1, n1, err := canonicalSPDXSHA256(first)
	if err != nil {
		t.Fatalf("canonicalSPDXSHA256(first) error: %v", err)
	}
	h2, n2, err := canonicalSPDXSHA256(second)
	if err != nil {
		t.Fatalf("canonicalSPDXSHA256(second) error: %v", err)
	}

	if h1 != h2 {
		t.Fatalf("expected canonical hash to be stable across order, got %q != %q", h1, h2)
	}
	if n1 != 2 || n2 != 2 {
		t.Fatalf("expected package count 2, got %d and %d", n1, n2)
	}
}

func TestCanonicalSPDXSHA256_DetectsContentChange(t *testing.T) {
	base := []byte(`{"packages":[{"name":"acl","versionInfo":"2.3.1"}]}`)
	changed := []byte(`{"packages":[{"name":"acl","versionInfo":"2.3.2"}]}`)

	h1, _, err := canonicalSPDXSHA256(base)
	if err != nil {
		t.Fatalf("canonicalSPDXSHA256(base) error: %v", err)
	}
	h2, _, err := canonicalSPDXSHA256(changed)
	if err != nil {
		t.Fatalf("canonicalSPDXSHA256(changed) error: %v", err)
	}

	if h1 == h2 {
		t.Fatalf("expected canonical hash to differ after content change")
	}
}

func TestCompareImages_SBOMAdded(t *testing.T) {
	from := &ImageSummary{}
	to := &ImageSummary{
		SBOM: SBOMSummary{
			Present:         true,
			FileName:        "spdx_manifest_deb_demo_20260101_000000.json",
			Format:          "spdx",
			CanonicalSHA256: "abc",
		},
	}

	res := CompareImages(from, to)
	if !res.Summary.SBOMChanged {
		t.Fatalf("expected Summary.SBOMChanged=true")
	}
	if res.Diff.SBOM == nil || !res.Diff.SBOM.Changed || res.Diff.SBOM.Added == nil {
		t.Fatalf("expected SBOM added diff, got %+v", res.Diff.SBOM)
	}
}

func TestCompareImages_SBOMCanonicalChanged(t *testing.T) {
	from := &ImageSummary{
		SBOM: SBOMSummary{
			Present:         true,
			FileName:        "spdx_manifest_rpm_demo_20260101_000000.json",
			Format:          "spdx",
			CanonicalSHA256: "aaa",
			PackageCount:    100,
		},
	}
	to := &ImageSummary{
		SBOM: SBOMSummary{
			Present:         true,
			FileName:        "spdx_manifest_rpm_demo_20260102_000000.json",
			Format:          "spdx",
			CanonicalSHA256: "bbb",
			PackageCount:    101,
		},
	}

	res := CompareImages(from, to)
	if res.Diff.SBOM == nil || !res.Diff.SBOM.Changed {
		t.Fatalf("expected SBOM diff, got %+v", res.Diff.SBOM)
	}
	if res.Diff.SBOM.CanonicalSHA256 == nil {
		t.Fatalf("expected canonical SBOM hash diff")
	}
	if res.Diff.SBOM.PackageCount == nil {
		t.Fatalf("expected SBOM package count diff")
	}
	if res.Equality.Class != EqualityDifferent {
		t.Fatalf("expected EqualityDifferent, got %s", res.Equality.Class)
	}
}

func TestCompareSPDXFiles(t *testing.T) {
	tmpDir := t.TempDir()
	fromPath := filepath.Join(tmpDir, "from.spdx.json")
	toPath := filepath.Join(tmpDir, "to.spdx.json")

	fromContent := `{"packages":[{"name":"acl","versionInfo":"2.3.1","downloadLocation":"https://example.com/acl.rpm"}]}`
	toContent := `{"packages":[{"name":"acl","versionInfo":"2.3.2","downloadLocation":"https://example.com/acl.rpm"}]}`

	if err := os.WriteFile(fromPath, []byte(fromContent), 0644); err != nil {
		t.Fatalf("write from SPDX file: %v", err)
	}
	if err := os.WriteFile(toPath, []byte(toContent), 0644); err != nil {
		t.Fatalf("write to SPDX file: %v", err)
	}

	result, err := CompareSPDXFiles(fromPath, toPath)
	if err != nil {
		t.Fatalf("CompareSPDXFiles error: %v", err)
	}

	if result.Equal {
		t.Fatalf("expected SPDX files to differ")
	}
	if result.FromPackageCount != 1 || result.ToPackageCount != 1 {
		t.Fatalf("expected package counts 1 and 1, got %d and %d", result.FromPackageCount, result.ToPackageCount)
	}
	if len(result.AddedPackages) != 1 || len(result.RemovedPackages) != 1 {
		t.Fatalf("expected one added and one removed package key, got added=%d removed=%d",
			len(result.AddedPackages), len(result.RemovedPackages))
	}
}
