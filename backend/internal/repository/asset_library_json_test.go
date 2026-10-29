package repository

import (
	"strings"
	"testing"
)

func TestUserAssetJSONExpressionsUseDatabaseDialect(t *testing.T) {
	t.Parallel()

	postgresFavorite := userAssetFavoriteSQL("postgres")
	if !strings.Contains(postgresFavorite, "#>>") || strings.Contains(postgresFavorite, "json_extract") {
		t.Fatalf("postgres favorite expression = %q", postgresFavorite)
	}

	postgresGenerated := userAssetGeneratedSQL("postgres")
	if !strings.Contains(postgresGenerated, "jsonb_typeof") || strings.Contains(postgresGenerated, "json_extract") {
		t.Fatalf("postgres generated expression = %q", postgresGenerated)
	}

	postgresProject := userAssetProjectLabelSQL("postgres")
	if !strings.Contains(postgresProject, "jsonb_array_length") || strings.Contains(postgresProject, "json_extract") {
		t.Fatalf("postgres project expression = %q", postgresProject)
	}

	sqliteFavorite := userAssetFavoriteSQL("sqlite")
	if !strings.Contains(sqliteFavorite, "json_extract") {
		t.Fatalf("sqlite favorite expression = %q", sqliteFavorite)
	}
}
