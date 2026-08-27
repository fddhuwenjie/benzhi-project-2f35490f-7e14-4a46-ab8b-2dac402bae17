package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type smokeClient struct {
	baseURL string
	client  *http.Client
	serial  int
}

func (c *smokeClient) command(method, path string, version int, payload map[string]any, target any) error {
	c.serial++
	payload["request_id"] = fmt.Sprintf("selfcheck-%03d", c.serial)
	payload["expected_version"] = version
	return c.request(method, path, payload, target)
}

func (c *smokeClient) get(path string, target any) error {
	return c.request(http.MethodGet, path, nil, target)
}

func (c *smokeClient) getBytes(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s 返回 %d: %s", path, res.StatusCode, string(raw))
	}
	return raw, nil
}

func (c *smokeClient) request(method, path string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d: %s", method, path, res.StatusCode, string(raw))
	}
	if target != nil && len(raw) != 0 {
		if err := json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("解析自检响应: %w", err)
		}
	}
	return nil
}

func newSmokeClient(address string) *smokeClient {
	return &smokeClient{baseURL: "http://" + address, client: &http.Client{Timeout: 8 * time.Second}}
}
