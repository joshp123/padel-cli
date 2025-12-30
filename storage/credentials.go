package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Credentials struct {
	AccessToken            string `json:"access_token"`
	AccessTokenExpiration  string `json:"access_token_expiration"`
	RefreshToken           string `json:"refresh_token"`
	RefreshTokenExpiration string `json:"refresh_token_expiration"`
	UserID                 string `json:"user_id"`
	Email                  string `json:"email"`
}

func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("credentials path is a directory: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var creds Credentials
	if err := json.NewDecoder(file).Decode(&creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

func SaveCredentials(creds *Credentials) error {
	if _, err := ensureConfigDir(); err != nil {
		return err
	}
	path, err := CredentialsPath()
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(creds)
}

func ClearCredentials() error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Credentials) AccessTokenExpired(now time.Time) bool {
	exp, err := parseCredentialTime(c.AccessTokenExpiration)
	if err != nil {
		return true
	}
	return now.UTC().After(exp)
}

func parseCredentialTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	layout := "2006-01-02T15:04:05"
	parsed, err := time.ParseInLocation(layout, value, time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}
