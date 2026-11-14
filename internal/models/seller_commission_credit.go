package models

import "time"

// Seller commission credit statuses.
const (
	CommissionCreditStatusPending = "pending"
	CommissionCreditStatusSettled = "settled"
)

// SellerCommissionCredit is a recoverable credit created when a contract whose
// commission was already paid to the seller is cancelled. It stays pending
// until an admin applies it.
type SellerCommissionCredit struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	SellerID   uint       `gorm:"column:seller_id;not null;index" json:"seller_id"`
	ContractID uint       `gorm:"column:contract_id;not null" json:"contract_id"`
	Amount     float64    `gorm:"column:amount;type:decimal(15,2);not null" json:"amount"`
	Status     string     `gorm:"column:status;size:20;not null;default:pending" json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	SettledAt  *time.Time `gorm:"column:settled_at" json:"settled_at"`
	SettledBy  *uint      `gorm:"column:settled_by" json:"settled_by,omitempty"`

	Seller   *User     `gorm:"foreignKey:SellerID" json:"seller,omitempty"`
	Contract *Contract `gorm:"foreignKey:ContractID" json:"contract,omitempty"`
}

// TableName specifies the table name for SellerCommissionCredit.
func (SellerCommissionCredit) TableName() string {
	return "seller_commission_credits"
}

// SellerCommissionCreditResponse is the JSON representation consumed by the
// commissions report and the admin credit actions. The seller is exposed by
// name so the UI can render it read-only.
type SellerCommissionCreditResponse struct {
	ID           uint       `json:"id"`
	SellerID     uint       `json:"seller_id"`
	Seller       string     `json:"seller"`
	ContractID   uint       `json:"contract_id"`
	ContractGUID string     `json:"contract_guid"`
	Amount       float64    `json:"amount"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	SettledAt    *time.Time `json:"settled_at"`
	SettledBy    *uint      `json:"settled_by,omitempty"`
}

// ToResponse converts a credit to its API representation.
func (c *SellerCommissionCredit) ToResponse() SellerCommissionCreditResponse {
	sellerName := ""
	if c.Seller != nil {
		sellerName = c.Seller.FullName
	}
	contractGUID := ""
	if c.Contract != nil {
		contractGUID = c.Contract.GUID
	}
	status := c.Status
	if status == "" {
		status = CommissionCreditStatusPending
	}
	return SellerCommissionCreditResponse{
		ID:           c.ID,
		SellerID:     c.SellerID,
		Seller:       sellerName,
		ContractID:   c.ContractID,
		ContractGUID: contractGUID,
		Amount:       c.Amount,
		Status:       status,
		CreatedAt:    c.CreatedAt,
		SettledAt:    c.SettledAt,
		SettledBy:    c.SettledBy,
	}
}
