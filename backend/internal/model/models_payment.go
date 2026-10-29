package model

import "time"

// Payment provider identifiers. A provider is the gateway family; a
// PaymentProviderConfig is one concrete merchant account of that family, so two
// Alipay merchant numbers can coexist and be switched without a deploy.
const (
	PaymentProviderAlipay = "alipay"
	PaymentProviderWeChat = "wechat"
	PaymentProviderEPay   = "epay"
)

// PaymentProviderConfig is an admin-managed payment channel. The non-secret
// fields are readable; every credential lives in SecretsEncrypted as one
// AES-GCM ciphertext so adding a field never needs a schema change and the
// plaintext never leaves the process.
type PaymentProviderConfig struct {
	ID               string     `json:"id" gorm:"primaryKey;size:36"`
	Provider         string     `json:"provider" gorm:"size:32;index"`
	Name             string     `json:"name" gorm:"size:128"`
	AppID            string     `json:"appId" gorm:"size:128"`
	MchID            string     `json:"mchId" gorm:"size:128"`
	MchSerialNo      string     `json:"mchSerialNo" gorm:"size:128"`
	NotifyURL        string     `json:"notifyUrl" gorm:"size:255"`
	ReturnURL        string     `json:"returnUrl" gorm:"size:255"`
	Endpoint         string     `json:"endpoint" gorm:"size:255"`
	PayType          string     `json:"payType" gorm:"size:24"`
	SecretsEncrypted string     `json:"-" gorm:"type:text"`
	Enabled          bool       `json:"enabled"`
	SortOrder        int        `json:"sortOrder"`
	LastTestedAt     *time.Time `json:"lastTestedAt"`
	LastTestError    string     `json:"lastTestError" gorm:"type:text"`
	CreatedBy        string     `json:"createdBy" gorm:"size:36"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

func (PaymentProviderConfig) TableName() string { return "payment_provider_configs" }
