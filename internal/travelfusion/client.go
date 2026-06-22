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

func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	startPayload, err := buildStartRoutingXML(c.xmlLoginID, c.loginID, c.timeoutSeconds, req)
	if err != nil {
		return nil, fmt.Errorf("build start routing request: %w", err)
	}
	startBody, err := c.postXML(ctx, "StartRouting", startPayload)
	if err != nil {
		return nil, fmt.Errorf("start routing: %w", err)
	}

	var startResp commandListStartRoutingResponse
	if err := xml.Unmarshal(startBody, &startResp); err != nil {
		return nil, fmt.Errorf("parse start routing response: %w", err)
	}
	if strings.TrimSpace(startResp.StartRouting.RoutingID) == "" {
		return nil, fmt.Errorf("travelfusion start routing returned empty routing id")
	}
	if len(startResp.StartRouting.RouterList) == 0 {
		return &SearchResult{RoutingID: startResp.StartRouting.RoutingID}, nil
	}

	result := &SearchResult{RoutingID: startResp.StartRouting.RoutingID}
	for attempt := 0; attempt < c.pollingAttempts; attempt++ {
		if attempt > 0 || c.pollingDelaySeconds > 0 {
			if err := sleepContext(ctx, time.Duration(c.pollingDelaySeconds)*time.Second); err != nil {
				return nil, err
			}
		}

		checkPayload, err := buildCheckRoutingXML(c.xmlLoginID, c.loginID, startResp.StartRouting.RoutingID)
		if err != nil {
			return nil, fmt.Errorf("build check routing request: %w", err)
		}
		checkBody, err := c.postXML(ctx, "CheckRouting", checkPayload)
		if err != nil {
			return nil, fmt.Errorf("check routing: %w", err)
		}

		var checkResp commandListCheckRoutingResponse
		if err := xml.Unmarshal(checkBody, &checkResp); err != nil {
			return nil, fmt.Errorf("parse check routing response: %w", err)
		}
		outward, returns := extractFlights(checkResp.CheckRouting)
		result.OutwardFlights = append(result.OutwardFlights, outward...)
		result.ReturnFlights = append(result.ReturnFlights, returns...)

		if routingComplete(checkResp.CheckRouting) {
			break
		}
	}

	return result, nil
}

func (c *Client) GetCurrencies(ctx context.Context) (map[string]Currency, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	payload, err := buildGetCurrenciesXML(c.xmlLoginID, c.loginID)
	if err != nil {
		return nil, fmt.Errorf("build get currencies request: %w", err)
	}
	body, err := c.postXML(ctx, "GetCurrencies", payload)
	if err != nil {
		return nil, fmt.Errorf("get currencies: %w", err)
	}

	var resp commandListGetCurrenciesResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse get currencies response: %w", err)
	}
	return mapCurrencies(resp.GetCurrencies), nil
}

func (c *Client) ProcessDetails(ctx context.Context, req ProcessDetailsRequest) (*ProcessDetailsResult, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	payload, err := buildProcessDetailsXML(c.xmlLoginID, c.loginID, req)
	if err != nil {
		return nil, fmt.Errorf("build process details request: %w", err)
	}
	body, err := c.postXML(ctx, "ProcessDetails", payload)
	if err != nil {
		return nil, fmt.Errorf("process details: %w", err)
	}

	var resp commandListProcessDetailsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse process details response: %w", err)
	}
	return mapProcessDetails(resp.ProcessDetails), nil
}

func (c *Client) GetBranchSupplierList(ctx context.Context) ([]string, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	payload, err := buildGetBranchSupplierListXML(c.xmlLoginID, c.loginID)
	if err != nil {
		return nil, fmt.Errorf("build get branch supplier list request: %w", err)
	}
	body, err := c.postXML(ctx, "GetBranchSupplierList", payload)
	if err != nil {
		return nil, fmt.Errorf("get branch supplier list: %w", err)
	}

	var resp commandListGetBranchSupplierListResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse get branch supplier list response: %w", err)
	}
	suppliers := make([]string, 0, len(resp.GetBranchSupplierList.Suppliers))
	for _, supplier := range resp.GetBranchSupplierList.Suppliers {
		supplier = strings.TrimSpace(supplier)
		if supplier != "" {
			suppliers = append(suppliers, supplier)
		}
	}
	return suppliers, nil
}

func (c *Client) ListSupplierRoutes(ctx context.Context, supplier string, oneWayOnlyAirportRoutes bool) (*SupplierRoutesResult, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	payload, err := buildListSupplierRoutesXML(c.xmlLoginID, c.loginID, supplier, oneWayOnlyAirportRoutes)
	if err != nil {
		return nil, fmt.Errorf("build list supplier routes request: %w", err)
	}
	body, err := c.postXML(ctx, "ListSupplierRoutes", payload)
	if err != nil {
		return nil, fmt.Errorf("list supplier routes: %w", err)
	}

	var resp commandListListSupplierRoutesResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse list supplier routes response: %w", err)
	}
	return &SupplierRoutesResult{
		AirportRoutes: parseRouteCodes(resp.ListSupplierRoutes.AirportRoutes),
		CityRoutes:    parseRouteCodes(resp.ListSupplierRoutes.CityRoutes),
	}, nil
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
