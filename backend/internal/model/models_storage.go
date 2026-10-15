package model

import "time"

// StorageLocation is a platform-level object-storage configuration owned by the
// admin backend. Credentials are stored encrypted and never returned in clear.
type StorageLocation struct {
	ID              string     `json:"id" gorm:"primaryKey;size:36"`
	Name            string     `json:"name" gorm:"size:120;index"`
	Provider        string     `json:"provider" gorm:"size:32;index"`
	Endpoint        string     `json:"endpoint" gorm:"size:500"`
	Region          string     `json:"region" gorm:"size:120"`
	Bucket          string     `json:"bucket" gorm:"size:255"`
	AccessKeyID     string     `json:"accessKeyId" gorm:"size:255"`
	SecretAccessKey string     `json:"-" gorm:"type:text"`
	CDNBaseURL      string     `json:"cdnBaseUrl" gorm:"size:500"`
	Prefix          string     `json:"prefix" gorm:"size:255"`
	PathStyle       bool       `json:"pathStyle"`
	IsDefault       bool       `json:"isDefault" gorm:"index"`
	Enabled         bool       `json:"enabled" gorm:"index"`
	LastTestedAt    *time.Time `json:"lastTestedAt"`
	LastTestError   string     `json:"lastTestError" gorm:"type:text"`
	CreatedBy       string     `json:"createdBy" gorm:"size:36;index"`
	UpdatedBy       string     `json:"updatedBy" gorm:"size:36"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (StorageLocation) TableName() string { return "storage_locations" }
