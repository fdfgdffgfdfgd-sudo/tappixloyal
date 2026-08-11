package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type PosterAdapter struct {
	client  *http.Client
	baseURL string
}

func NewPosterAdapter(client *http.Client, baseURL string) *PosterAdapter {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://joinposter.com/api"
	}
	return &PosterAdapter{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

type posterEnvelope struct {
	Response json.RawMessage `json:"response"`
	Error    any             `json:"error"`
}

func (p *PosterAdapter) request(ctx context.Context, connection Connection, method string, query url.Values, target any) error {
	token := strings.TrimSpace(connection.Credentials["accessToken"])
	if token == "" {
		token = strings.TrimSpace(connection.Credentials["token"])
	}
	if token == "" {
		return errors.New("Poster access token is required")
	}
	query.Set("token", token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/"+method+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Tappix-Poster/1.0")
	response, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Poster returned HTTP %d", response.StatusCode)
	}
	var envelope posterEnvelope
	if err = json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("invalid Poster response: %w", err)
	}
	if envelope.Error != nil || len(envelope.Response) == 0 || string(envelope.Response) == "null" {
		return fmt.Errorf("Poster API error: %v", envelope.Error)
	}
	return json.Unmarshal(envelope.Response, target)
}

func (p *PosterAdapter) Authorize(_ context.Context, input AuthorizationInput) error {
	if strings.TrimSpace(input.Credentials["accessToken"]) == "" && strings.TrimSpace(input.Credentials["token"]) == "" {
		return errors.New("Poster access token is required")
	}
	return nil
}

func (p *PosterAdapter) TestConnection(ctx context.Context, connection Connection) error {
	_, err := p.ListLocations(ctx, connection)
	return err
}

func (p *PosterAdapter) ListLocations(ctx context.Context, connection Connection) ([]Location, error) {
	var records []struct {
		ID   string `json:"spot_id"`
		Name string `json:"spot_name"`
	}
	if err := p.request(ctx, connection, "spots.getSpots", url.Values{}, &records); err != nil {
		return nil, err
	}
	items := make([]Location, 0, len(records))
	for _, record := range records {
		items = append(items, Location{ExternalID: record.ID, Name: record.Name})
	}
	return items, nil
}

func (p *PosterAdapter) ImportCustomers(ctx context.Context, connection Connection, cursor string) (CustomerBatch, error) {
	offset, _ := strconv.Atoi(cursor)
	var records []struct {
		ID        string `json:"client_id"`
		FirstName string `json:"firstname"`
		LastName  string `json:"lastname"`
		Phone     string `json:"phone"`
	}
	query := url.Values{"num": {"100"}, "offset": {strconv.Itoa(offset)}}
	if err := p.request(ctx, connection, "clients.getClients", query, &records); err != nil {
		return CustomerBatch{}, err
	}
	items := make([]CanonicalCustomer, 0, len(records))
	for _, record := range records {
		items = append(items, CanonicalCustomer{ExternalID: record.ID, Phone: NormalizePhone(record.Phone), FirstName: record.FirstName, LastName: record.LastName})
	}
	next := ""
	if len(records) == 100 {
		next = strconv.Itoa(offset + len(records))
	}
	return CustomerBatch{Customers: items, NextCursor: next}, nil
}

func posterMoney(value string) float64 {
	number, _ := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	return number / 100
}

func posterTime(value string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func (p *PosterAdapter) ImportTransactions(ctx context.Context, connection Connection, cursor string) (TransactionBatch, error) {
	dateFrom := time.Now().AddDate(0, 0, -90).Format("20060102")
	if cursor != "" {
		dateFrom = cursor
	}
	var records []map[string]any
	query := url.Values{"dateFrom": {dateFrom}, "dateTo": {time.Now().Format("20060102")}}
	if err := p.request(ctx, connection, "transactions.getTransactions", query, &records); err != nil {
		return TransactionBatch{}, err
	}
	items := make([]CanonicalTransaction, 0, len(records))
	for _, record := range records {
		transaction := canonicalPosterTransaction(connection, record)
		transaction.Source = "poster_import"
		items = append(items, transaction)
	}
	return TransactionBatch{Transactions: items, NextCursor: time.Now().AddDate(0, 0, -1).Format("20060102")}, nil
}

func (p *PosterAdapter) ResolveWebhook(context.Context, WebhookRequest) ([]CanonicalEvent, error) {
	return nil, errors.New("Poster webhook parsing is not enabled in read-only mode")
}

func (p *PosterAdapter) GetTransaction(ctx context.Context, connection Connection, externalID string) (CanonicalTransaction, error) {
	var records []map[string]any
	query := url.Values{"transaction_id": {externalID}}
	if err := p.request(ctx, connection, "transactions.getTransaction", query, &records); err != nil {
		// Some Poster accounts return an object rather than a one-element array.
		var record map[string]any
		if secondErr := p.request(ctx, connection, "transactions.getTransaction", query, &record); secondErr != nil {
			return CanonicalTransaction{}, err
		}
		records = []map[string]any{record}
	}
	if len(records) == 0 {
		return CanonicalTransaction{}, errors.New("Poster transaction not found")
	}
	return canonicalPosterTransaction(connection, records[0]), nil
}

func canonicalPosterTransaction(connection Connection, record map[string]any) CanonicalTransaction {
	raw, _ := json.Marshal(record)
	text := func(key string) string {
		if record[key] == nil {
			return ""
		}
		return fmt.Sprint(record[key])
	}
	occurred := posterTime(text("date_close"))
	if occurred.IsZero() {
		occurred = posterTime(text("date_start"))
	}
	status := "completed"
	if value := strings.ToLower(text("status")); strings.Contains(value, "delete") || strings.Contains(value, "cancel") || strings.Contains(value, "return") || strings.Contains(value, "refund") {
		status = "cancelled"
	}
	gross := posterMoney(text("sum"))
	net := posterMoney(text("payed_sum"))
	if net == 0 {
		net = gross
	}
	return CanonicalTransaction{CompanyID: connection.CompanyID, ConnectionID: connection.ID, Provider: "poster", ExternalID: text("transaction_id"), ExternalLocationID: text("spot_id"), ExternalCustomerID: text("client_id"), Status: status, OccurredAt: occurred, GrossAmount: gross, NetAmount: net, CashPaidAmount: net, Currency: "KZT", ReceiptNumber: text("transaction_id"), Source: "poster", RawPayload: raw}
}
