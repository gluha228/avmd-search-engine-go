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
	result := &SearchResult{}
	for update := range c.SearchStream(ctx, req) {
		if update.Err != nil {
			return nil, update.Err
		}
		if strings.TrimSpace(update.RoutingID) != "" {
			result.RoutingID = update.RoutingID
		}
		result.OutwardFlights = append(result.OutwardFlights, update.OutwardFlights...)
		result.ReturnFlights = append(result.ReturnFlights, update.ReturnFlights...)
	}
	return result, nil
}

func (c *Client) SearchStream(ctx context.Context, req SearchRequest) <-chan SearchUpdate {
	updates := make(chan SearchUpdate, c.pollingAttempts)
	go func() {
		defer close(updates)
		c.searchStream(ctx, req, updates)
	}()
	return updates
}

func (c *Client) searchStream(ctx context.Context, req SearchRequest, updates chan<- SearchUpdate) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		sendSearchError(ctx, updates, ErrMissingCredentials)
		return
	}

	startPayload, err := buildStartRoutingXML(c.xmlLoginID, c.loginID, c.timeoutSeconds, req)
	if err != nil {
		sendSearchError(ctx, updates, fmt.Errorf("build start routing request: %w", err))
		return
	}
	startBody, err := c.postXML(ctx, "StartRouting", startPayload)
	if err != nil {
		sendSearchError(ctx, updates, fmt.Errorf("start routing: %w", err))
		return
	}

	var startResp commandListStartRoutingResponse
	if err := xml.Unmarshal(startBody, &startResp); err != nil {
		sendSearchError(ctx, updates, fmt.Errorf("parse start routing response: %w", err))
		return
	}
	if strings.TrimSpace(startResp.StartRouting.RoutingID) == "" {
		sendSearchError(ctx, updates, fmt.Errorf("travelfusion start routing returned empty routing id"))
		return
	}
	routingID := startResp.StartRouting.RoutingID
	if c.logger != nil {
		c.logger.Debug(
			"travelfusion search routers started",
			"routing_id", routingID,
			"routers_started", len(startResp.StartRouting.RouterList),
		)
	}
	if !sendSearchUpdate(ctx, updates, SearchUpdate{RoutingID: routingID}) {
		return
	}
	if !routingNeedsPolling(startResp.StartRouting.RouterList) {
		return
	}

	for attempt := 0; attempt < c.pollingAttempts; attempt++ {
		if attempt > 0 && c.pollingDelaySeconds > 0 {
			if err := sleepContext(ctx, time.Duration(c.pollingDelaySeconds)*time.Second); err != nil {
				sendSearchError(ctx, updates, err)
				return
			}
		}
		if c.logger != nil {
			c.logger.Debug(
				"Poll attempt",
				"attempt", attempt+1,
				"max_attempts", c.pollingAttempts,
				"routing_id", routingID,
			)
		}

		checkPayload, err := buildCheckRoutingXML(c.xmlLoginID, c.loginID, routingID)
		if err != nil {
			sendSearchError(ctx, updates, fmt.Errorf("build check routing request: %w", err))
			return
		}
		checkBody, err := c.postXML(ctx, "CheckRouting", checkPayload)
		if err != nil {
			sendSearchError(ctx, updates, fmt.Errorf("check routing: %w", err))
			return
		}

		var checkResp commandListCheckRoutingResponse
		if err := xml.Unmarshal(checkBody, &checkResp); err != nil {
			sendSearchError(ctx, updates, fmt.Errorf("parse check routing response: %w", err))
			return
		}
		if c.logger != nil {
			c.logger.Debug(
				"TravelFusion search routers completed",
				"attempt", attempt+1,
				"max_attempts", c.pollingAttempts,
				"routing_id", routingID,
				"completed_routers", completedRouterCount(checkResp.CheckRouting.RouterList),
				"total_routers", len(checkResp.CheckRouting.RouterList),
			)
		}
		outward, returns := extractFlights(checkResp.CheckRouting)
		if len(outward) > 0 || len(returns) > 0 {
			if !sendSearchUpdate(ctx, updates, SearchUpdate{
				RoutingID:      routingID,
				OutwardFlights: outward,
				ReturnFlights:  returns,
			}) {
				return
			}
		}

		if routingComplete(checkResp.CheckRouting) {
			break
		}
	}
}

func sendSearchError(ctx context.Context, updates chan<- SearchUpdate, err error) {
	sendSearchUpdate(ctx, updates, SearchUpdate{Err: err})
}

func sendSearchUpdate(ctx context.Context, updates chan<- SearchUpdate, update SearchUpdate) bool {
	select {
	case <-ctx.Done():
		return false
	case updates <- update:
		return true
	}
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

func (c *Client) ProcessTerms(ctx context.Context, req ProcessTermsRequest) (*ProcessTermsResult, error) {
	if strings.TrimSpace(c.xmlLoginID) == "" || strings.TrimSpace(c.loginID) == "" {
		return nil, ErrMissingCredentials
	}

	payload, err := buildProcessTermsXML(c.xmlLoginID, c.loginID, req)
	if err != nil {
		return nil, fmt.Errorf("build process terms request: %w", err)
	}
	body, err := c.postXML(ctx, "ProcessTerms", payload)
	if err != nil {
		return nil, fmt.Errorf("process terms: %w", err)
	}

	var resp commandListProcessTermsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse process terms response: %w", err)
	}
	return mapProcessTerms(resp.ProcessTerms), nil
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
