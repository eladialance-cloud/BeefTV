package appearance

import "testing"

func TestNormalizeDocumentMigratesLegacyBrand(t *testing.T) {
	value := DefaultSetting()
	value.BrandName = LegacyBrandName
	value.BrandSlug = LegacyBrandSlug
	value.LogoResourceID = "legacy-logo"
	value.DarkLogoResourceID = "legacy-dark-logo"
	value.SEOTitle = LegacyBrandName
	value.SEODescription = "使用" + LegacyBrandName + "创作"
	value.FooterCopyright = LegacyBrandName + " 版权所有"

	got := normalizeDocument(value)
	if got.BrandName != DefaultBrandName || got.BrandSlug != DefaultBrandSlug {
		t.Fatalf("brand = %q %q, want %q %q", got.BrandName, got.BrandSlug, DefaultBrandName, DefaultBrandSlug)
	}
	if got.LogoResourceID != "" || got.DarkLogoResourceID != "" {
		t.Fatalf("legacy logo ids were retained: %#v", got)
	}
	if got.SEOTitle != DefaultBrandName || got.SEODescription != "使用"+DefaultBrandName+"创作" || got.FooterCopyright != DefaultBrandName+" 版权所有" {
		t.Fatalf("legacy brand copy was retained: %#v", got)
	}
}
