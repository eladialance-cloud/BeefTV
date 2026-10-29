package app

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestCreationCatalogCRUD(t *testing.T) {
	svc, db := newChannelModelTestService(t)
	if err := db.AutoMigrate(&model.CreationCatalogItem{}); err != nil {
		t.Fatal(err)
	}
	admin := &model.User{ID: "admin", Role: model.UserRoleAdmin}
	enabled := true
	created, err := svc.CreateCreationCatalogItem(admin, CreationCatalogRequest{
		Kind:    CreationCatalogKindWorkflow,
		Name:    "电商出图",
		Scene:   "ecommerce",
		Enabled: &enabled,
		Payload: map[string]any{"nodes": []any{"input", "image"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.ItemKey == "" || created.Kind != CreationCatalogKindWorkflow {
		t.Fatalf("created item = %#v", created)
	}

	page, err := svc.AdminCreationCatalogPage(admin, CreationCatalogKindWorkflow, AdminListQuery{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Name != "电商出图" {
		t.Fatalf("page = %#v", page)
	}

	updated, err := svc.UpdateCreationCatalogItem(admin, created.ID, CreationCatalogRequest{
		Name:      "电商出图 - 新",
		Scene:     "ecommerce",
		Enabled:   &enabled,
		SortOrder: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "电商出图 - 新" || updated.SortOrder != 8 {
		t.Fatalf("updated item = %#v", updated)
	}

	if err := svc.DeleteCreationCatalogItem(admin, created.ID); err != nil {
		t.Fatal(err)
	}
	page, err = svc.AdminCreationCatalogPage(admin, CreationCatalogKindWorkflow, AdminListQuery{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 {
		t.Fatalf("deleted item still in page: %#v", page)
	}
}

func TestCreationCatalogValidatesKindAndRequiredContent(t *testing.T) {
	svc, db := newChannelModelTestService(t)
	if err := db.AutoMigrate(&model.CreationCatalogItem{}); err != nil {
		t.Fatal(err)
	}
	admin := &model.User{ID: "admin", Role: model.UserRoleAdmin}
	if _, err := svc.CreateCreationCatalogItem(admin, CreationCatalogRequest{Kind: "unknown", Name: "x"}); err == nil {
		t.Fatal("unknown kind should fail")
	}
	if _, err := svc.CreateCreationCatalogItem(admin, CreationCatalogRequest{Kind: CreationCatalogKindPrompt, Name: "x"}); err == nil {
		t.Fatal("prompt without content should fail")
	}
	if _, err := svc.CreateCreationCatalogItem(admin, CreationCatalogRequest{Kind: CreationCatalogKindStyle, Name: "x"}); err == nil {
		t.Fatal("style without prefix should fail")
	}
}
