package apm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Register creates an account on a registry node. The returned token is shown
// exactly once: the node stores only its hash, so it cannot be retrieved later.
func (c *Client) Register(username, email, password string) (*AuthResponse, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("registration requires a username and a password")
	}
	body, err := json.Marshal(RegisterRequest{
		Username: strings.TrimSpace(username),
		Email:    strings.TrimSpace(email),
		Password: password,
	})
	if err != nil {
		return nil, err
	}
	return c.authCall(APIBasePath+"/register", body)
}

// Login exchanges a username and password for a new one-time token. Use it when
// the token from registration was lost.
func (c *Client) Login(username, password string) (*AuthResponse, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("login requires a username and a password")
	}
	body, err := json.Marshal(LoginRequest{Username: strings.TrimSpace(username), Password: password})
	if err != nil {
		return nil, err
	}
	return c.authCall(APIBasePath+"/login", body)
}

// Account returns the identity and tokens behind the client's token. It
// requires the client to be authenticated.
func (c *Client) Account() (*AccountResponse, error) {
	resp, err := c.do(http.MethodGet, APIBasePath+"/account", nil, true)
	if err != nil {
		return nil, err
	}
	var out AccountResponse
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Logout revokes the token this client authenticates with.
func (c *Client) Logout() error {
	resp, err := c.do(http.MethodPost, APIBasePath+"/logout", nil, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readHTTPError(resp)
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) authCall(path string, body []byte) (*AuthResponse, error) {
	resp, err := c.do(http.MethodPost, path, func() (io.Reader, string, error) {
		return bytes.NewReader(body), "application/json", nil
	}, false)
	if err != nil {
		return nil, err
	}
	var out AuthResponse
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	if out.Token == "" {
		return nil, fmt.Errorf("registry %s returned an account response without a token", c.base)
	}
	return &out, nil
}
