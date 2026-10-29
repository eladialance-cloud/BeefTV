package model

import "time"

// CreationCatalogItem 是后台配置、用户端读取的创作模板目录项。
// 用一张表承载工作流模板、提示词和画风，避免三个几乎相同的 CRUD 面重复漂移；
// kind 决定字段语义，payload_json 给流程定义等后续扩展留出空间。
type CreationCatalogItem struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	Kind         string    `json:"kind" gorm:"size:24;index;uniqueIndex:idx_creation_catalog_kind_key,priority:1"`
	ItemKey      string    `json:"itemKey" gorm:"size:120;uniqueIndex:idx_creation_catalog_kind_key,priority:2"`
	Name         string    `json:"name" gorm:"size:160"`
	Scene        string    `json:"scene" gorm:"size:24;index"`
	Group        string    `json:"group" gorm:"size:64;index"`
	Description  string    `json:"description" gorm:"type:text"`
	Content      string    `json:"content" gorm:"type:text"`
	Variables    string    `json:"variables" gorm:"type:text"`
	CanvasType   string    `json:"canvasType" gorm:"size:24"`
	DefaultModel string    `json:"defaultModel" gorm:"size:160"`
	CoverURL     string    `json:"coverUrl" gorm:"size:500"`
	PayloadJSON  string    `json:"payloadJson" gorm:"type:text"`
	SortOrder    int       `json:"sortOrder" gorm:"index"`
	Enabled      bool      `json:"enabled" gorm:"index"`
	CreatedBy    string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (CreationCatalogItem) TableName() string {
	return "creation_catalog_items"
}
