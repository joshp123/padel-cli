package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

type PaymentIntentRequest struct {
	AllowedPaymentMethodTypes []string          `json:"allowed_payment_method_types"`
	UserID                    string            `json:"user_id"`
	Cart                      PaymentIntentCart `json:"cart"`
}

type PaymentIntentCart struct {
	RequestedItem PaymentIntentItem `json:"requested_item"`
}

type PaymentIntentItem struct {
	CartItemType      string                `json:"cart_item_type"`
	CartItemVoucherID any                   `json:"cart_item_voucher_id"`
	CartItemData      PaymentIntentItemData `json:"cart_item_data"`
}

type PaymentIntentItemData struct {
	SupportsSplitPayment bool                `json:"supports_split_payment"`
	NumberOfPlayers      int                 `json:"number_of_players"`
	TenantID             string              `json:"tenant_id"`
	ResourceID           string              `json:"resource_id"`
	Start                string              `json:"start"`
	Duration             int                 `json:"duration"`
	MatchRegistrations   []MatchRegistration `json:"match_registrations"`
}

type MatchRegistration struct {
	UserID string `json:"user_id"`
	PayNow bool   `json:"pay_now"`
}

type PaymentIntentResponse struct {
	PaymentIntentID         string `json:"payment_intent_id"`
	AvailablePaymentMethods []any  `json:"available_payment_methods"`
}

type PaymentIntentUpdateRequest struct {
	SelectedPaymentMethod string `json:"selected_payment_method"`
}

func (c *Client) CreatePaymentIntent(ctx context.Context, payload PaymentIntentRequest) (PaymentIntentResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return PaymentIntentResponse{}, err
	}
	req, err := c.newAPIRequest(ctx, "POST", "/payment_intents", nil)
	if err != nil {
		return PaymentIntentResponse{}, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))

	var resp PaymentIntentResponse
	if err := c.doJSON(req, &resp); err != nil {
		return PaymentIntentResponse{}, err
	}
	if resp.PaymentIntentID == "" {
		return PaymentIntentResponse{}, fmt.Errorf("payment intent missing id")
	}
	return resp, nil
}

func (c *Client) UpdatePaymentIntent(ctx context.Context, paymentIntentID string, payload PaymentIntentUpdateRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	path := "/payment_intents/" + url.PathEscape(paymentIntentID)
	req, err := c.newAPIRequest(ctx, "PATCH", path, nil)
	if err != nil {
		return err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))

	return c.doStatus(req)
}

func (c *Client) ConfirmPaymentIntent(ctx context.Context, paymentIntentID string) (map[string]any, error) {
	path := "/payment_intents/" + url.PathEscape(paymentIntentID) + "/confirmation"
	req, err := c.newAPIRequest(ctx, "POST", path, nil)
	if err != nil {
		return nil, err
	}

	var resp map[string]any
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) GetMatches(ctx context.Context, size int, sort string, ownerID string) ([]Match, error) {
	q := url.Values{}
	q.Set("size", fmt.Sprintf("%d", size))
	q.Set("sort", sort)
	q.Set("owner_id", ownerID)

	req, err := c.newAPIRequest(ctx, "GET", "/matches", q)
	if err != nil {
		return nil, err
	}

	var matches []Match
	if err := c.doJSON(req, &matches); err != nil {
		return nil, err
	}
	return matches, nil
}
