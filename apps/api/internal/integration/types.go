package integration

import (
	"context"
	"encoding/json"
	"time"
)

type AuthorizationInput struct {
	Credentials map[string]string `json:"credentials"`
}

type Connection struct {
	ID                string
	CompanyID         string
	Provider          string
	ExternalAccountID string
	Config            json.RawMessage
	Credentials       map[string]string
}

type Location struct {
	ExternalID string `json:"externalId"`
	Name       string `json:"name"`
}

type CustomerBatch struct {
	Customers  []CanonicalCustomer `json:"customers"`
	NextCursor string              `json:"nextCursor"`
}

type TransactionBatch struct {
	Transactions []CanonicalTransaction `json:"transactions"`
	NextCursor   string                 `json:"nextCursor"`
}

type WebhookRequest struct {
	Headers map[string]string
	Body    []byte
}

type CanonicalEvent struct {
	ID          string                `json:"id"`
	Type        string                `json:"type"`
	Transaction *CanonicalTransaction `json:"transaction,omitempty"`
}

type POSAdapter interface {
	Authorize(context.Context, AuthorizationInput) error
	TestConnection(context.Context, Connection) error
	ListLocations(context.Context, Connection) ([]Location, error)
	ImportCustomers(context.Context, Connection, string) (CustomerBatch, error)
	ImportTransactions(context.Context, Connection, string) (TransactionBatch, error)
	ResolveWebhook(context.Context, WebhookRequest) ([]CanonicalEvent, error)
	GetTransaction(context.Context, Connection, string) (CanonicalTransaction, error)
}

type CanonicalCustomer struct {
	ExternalID string `json:"externalId"`
	Phone      string `json:"phone"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
}

type CanonicalItem struct {
	ExternalID        string         `json:"externalId"`
	ProductExternalID string         `json:"productExternalId"`
	SKU               string         `json:"sku"`
	Name              string         `json:"name"`
	Category          string         `json:"category"`
	Quantity          float64        `json:"quantity"`
	UnitPrice         float64        `json:"unitPrice"`
	GrossAmount       float64        `json:"grossAmount"`
	DiscountAmount    float64        `json:"discountAmount"`
	NetAmount         float64        `json:"netAmount"`
	CostAmount        *float64       `json:"costAmount,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type CanonicalPayment struct {
	Provider   string         `json:"provider"`
	ExternalID string         `json:"externalId"`
	Type       string         `json:"type"`
	Status     string         `json:"status"`
	Amount     float64        `json:"amount"`
	OccurredAt time.Time      `json:"occurredAt"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type CanonicalTransaction struct {
	CompanyID          string             `json:"-"`
	ConnectionID       string             `json:"connectionId"`
	BranchID           string             `json:"branchId"`
	CustomerID         string             `json:"customerId"`
	Provider           string             `json:"provider"`
	ExternalID         string             `json:"externalId"`
	ExternalLocationID string             `json:"externalLocationId"`
	ExternalCustomerID string             `json:"externalCustomerId"`
	CustomerPhone      string             `json:"customerPhone"`
	Status             string             `json:"status"`
	OccurredAt         time.Time          `json:"occurredAt"`
	GrossAmount        float64            `json:"grossAmount"`
	DiscountAmount     float64            `json:"discountAmount"`
	BonusPaidAmount    float64            `json:"bonusPaidAmount"`
	CashPaidAmount     float64            `json:"cashPaidAmount"`
	NetAmount          float64            `json:"netAmount"`
	CostAmount         *float64           `json:"costAmount,omitempty"`
	Currency           string             `json:"currency"`
	ReceiptNumber      string             `json:"receiptNumber"`
	Source             string             `json:"source"`
	CampaignID         string             `json:"campaignId"`
	OriginalExternalID string             `json:"originalExternalId"`
	BonusEarned        int                `json:"bonusEarned"`
	BonusSpent         int                `json:"bonusSpent"`
	Items              []CanonicalItem    `json:"items"`
	Payments           []CanonicalPayment `json:"payments"`
	RawPayload         json.RawMessage    `json:"rawPayload"`
	Metadata           map[string]any     `json:"metadata,omitempty"`
	Sandbox            bool               `json:"sandbox"`
}

type IngestResult struct {
	TransactionID string `json:"transactionId"`
	CustomerID    string `json:"customerId,omitempty"`
	Duplicate     bool   `json:"duplicate"`
	VisitID       string `json:"visitId,omitempty"`
}

type RefundInput struct {
	CompanyID  string  `json:"-"`
	OriginalID string  `json:"-"`
	ExternalID string  `json:"externalId"`
	Amount     float64 `json:"amount"`
	Reason     string  `json:"reason"`
	Sandbox    bool    `json:"sandbox"`
}

type RefundResult struct {
	TransactionID   string  `json:"transactionId"`
	OriginalID      string  `json:"originalTransactionId"`
	RefundedAmount  float64 `json:"refundedAmount"`
	RemainingAmount float64 `json:"remainingAmount"`
	OriginalStatus  string  `json:"originalStatus"`
	BonusRestored   int     `json:"bonusRestored"`
	BonusReversed   int     `json:"bonusReversed"`
}
