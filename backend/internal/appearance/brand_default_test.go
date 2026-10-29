package appearance

import "testing"

func TestDefaultSettingUsesVisiDeepBrand(t *testing.T) {
	got := DefaultSetting()
	if got.BrandName != "VisiDeep AI" || got.BrandSlug != "visideep-ai" {
		t.Fatalf("default brand = %q %q, want VisiDeep AI visideep-ai", got.BrandName, got.BrandSlug)
	}
	public := publicSetting(nil, got)
	if public.LogoURL != "/visideep-ai.png" || public.DarkLogoURL != "/visideep-ai.png" {
		t.Fatalf("default logo = %q %q, want bundled VisiDeep logo", public.LogoURL, public.DarkLogoURL)
	}
}
