package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type AuthResponse struct {
	AccessToken            string `json:"access_token"`
	AccessTokenExpiration  string `json:"access_token_expiration"`
	RefreshToken           string `json:"refresh_token"`
	RefreshTokenExpiration string `json:"refresh_token_expiration"`
	UserID                 string `json:"user_id"`
}

func (c *Client) Login(ctx context.Context, email, password string) (AuthResponse, error) {
	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return AuthResponse{}, err
	}

	req, err := c.newRequest(ctx, c.AuthBaseURL, "POST", "/auth/login", nil, bytes.NewReader(body), false)
	if err != nil {
		return AuthResponse{}, err
	}

	var resp AuthResponse
	if err := c.doJSON(req, &resp); err != nil {
		return AuthResponse{}, err
	}
	if resp.AccessToken == "" {
		return AuthResponse{}, fmt.Errorf("login failed: missing access_token")
	}

	c.AccessToken = resp.AccessToken
	return resp, nil
}
