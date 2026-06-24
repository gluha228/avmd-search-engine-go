package travelfusion

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

var ErrMissingCredentials = errors.New("travelfusion credentials are not configured")

type Client struct {
	baseURL             string
	xmlLoginID          string
	loginID             string
	httpClient          *http.Client
	pollingAttempts     int
	pollingDelaySeconds int
	timeoutSeconds      int
	logger              *slog.Logger
}

type Config struct {
	BaseURL             string
	XmlLoginID          string
	LoginID             string
	TimeoutSeconds      int
	PollingAttempts     int
	PollingDelaySeconds int
}

func NewClient(cfg Config, logger *slog.Logger) *Client {
	return &Client{
		baseURL:             cfg.BaseURL,
		xmlLoginID:          cfg.XmlLoginID,
		loginID:             cfg.LoginID,
		httpClient:          &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
		pollingAttempts:     cfg.PollingAttempts,
		pollingDelaySeconds: cfg.PollingDelaySeconds,
		timeoutSeconds:      cfg.TimeoutSeconds,
		logger:              logger,
	}
}

func (c *Client) postXML(ctx context.Context, command string, payload []byte) ([]byte, error) {
	body := append([]byte(xml.Header), payload...)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "text/xml; charset=utf-8")
	httpReq.Header.Set("Accept", "text/xml, application/xml")
	if c.logger != nil {
		c.logger.Debug("travelfusion xml request sending", "command", command, "bytes", len(body))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if c.logger != nil {
			c.logger.Warn("travelfusion xml request failed", "command", command, "error", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if c.logger != nil {
			c.logger.Warn(
				"travelfusion xml response returned non-success status",
				"command", command,
				"status", resp.StatusCode,
				"bytes", len(respBody),
			)
		}
		return nil, fmt.Errorf("travelfusion returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if c.logger != nil {
		c.logger.Debug(
			"travelfusion xml response received",
			"command", command,
			"status", resp.StatusCode,
			"bytes", len(respBody),
		)
	}
	return respBody, nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
