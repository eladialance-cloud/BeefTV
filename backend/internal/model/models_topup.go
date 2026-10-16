package model

import "time"

// TopupProduct is a hosted recharge SKU shown on the user-facing top-up page.
// Price is in fen (分) so money never uses floating point; Credits is how many
// points the user receives after the payment settles.
type TopupProduct struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	Name      string    `json:"name" gorm:"size:128"`
	PriceFen  int64     `json:"priceFen"`
	Credits   int64     `json:"credits"`
	Bonus     int64     `json:"bonus"`
	Provider  string    `json:"provider" gorm:"size:32"`
	Enabled   bool      `json:"enabled"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (TopupProduct) TableName() string { return "topup_products" }

// TotalCredits is what the buyer actually receives: base credits plus bonus.
func (p TopupProduct) TotalCredits() int64 { return p.Credits + p.Bonus }

// Payment order status. A new order is pending_payment until a provider
// callback or an admin confirmation marks it paid; expired orders are closed.
const (
	PaymentOrderPendingPayment = "pending_payment"
	PaymentOrderPaid           = "paid"
	PaymentOrderClosed         = "closed"
)

// PaymentOrder is one recharge attempt. Credits snapshots the product's total
// at order time so later price changes never rewrite historical receipts.
// SettledAt and SettledBy are filled when the points are actually credited.
type PaymentOrder struct {
	ID                string     `json:"id" gorm:"primaryKey;size:36"`
	UserID            string     `json:"userId" gorm:"size:36;index"`
	ProductID         string     `json:"productId" gorm:"size:36;index"`
	ProductName       string     `json:"productName" gorm:"size:128"`
	Provider          string     `json:"provider" gorm:"size:32;index"`
	ProviderConfigID  string     `json:"providerConfigId" gorm:"size:36;index"`
	AmountFen         int64      `json:"amountFen"`
	Credits           int64      `json:"credits"`
	Status            string     `json:"status" gorm:"size:24;index"`
	ProviderTradeNo   string     `json:"providerTradeNo" gorm:"size:128"`
	CreatedAt         time.Time  `json:"createdAt" gorm:"index"`
	PaidAt            *time.Time `json:"paidAt"`
	SettledAt         *time.Time `json:"settledAt"`
	SettledBy         string     `json:"settledBy" gorm:"size:36"`
	CloseReason       string     `json:"closeReason" gorm:"type:text"`
	CreditIdempotency string     `json:"-" gorm:"size:191;index"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func (PaymentOrder) TableName() string { return "payment_orders" }

// PaymentNotification records every provider callback we received. Provider and
// ProviderNotificationID are unique together, so a retried callback converges
// on one row and can never credit twice.
type PaymentNotification struct {
	ID                     string    `json:"id" gorm:"primaryKey;size:36"`
	Provider               string    `json:"provider" gorm:"size:32;uniqueIndex:idx_payment_notification_provider_id,priority:1"`
	ProviderNotificationID string    `json:"providerNotificationId" gorm:"size:128;uniqueIndex:idx_payment_notification_provider_id,priority:2"`
	ProviderTradeNo        string    `json:"providerTradeNo" gorm:"size:128"`
	OrderID                string    `json:"orderId" gorm:"size:36;index"`
	Status                 string    `json:"status" gorm:"size:24"`
	AmountFen              int64     `json:"amountFen"`
	PayloadJSON            string    `json:"payloadJson" gorm:"type:text"`
	Processed              bool      `json:"processed"`
	Error                  string    `json:"error" gorm:"type:text"`
	CreatedAt              time.Time `json:"createdAt"`
}

func (PaymentNotification) TableName() string { return "payment_notifications" }
