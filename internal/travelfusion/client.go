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
	startBody, err := c.postXML(ctx, startPayload)
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
		checkBody, err := c.postXML(ctx, checkPayload)
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

func (c *Client) postXML(ctx context.Context, payload []byte) ([]byte, error) {
	body := append([]byte(xml.Header), payload...)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "text/xml; charset=utf-8")
	httpReq.Header.Set("Accept", "text/xml, application/xml")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("travelfusion returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if c.logger != nil {
		c.logger.Debug("travelfusion xml response received", "bytes", len(respBody))
	}
	return respBody, nil
}

func extractFlights(resp checkRoutingResponse) ([]Flight, []Flight) {
	var outward []Flight
	var returns []Flight
	for _, router := range resp.RouterList {
		groups := append([]group{}, router.GroupList...)
		groups = append(groups, router.Groups...)
		for _, group := range groups {
			for _, flight := range group.OutwardList {
				outward = append(outward, convertFlight(flight, group.Price, len(group.ReturnList) > 0, false))
			}
			for _, flight := range group.ReturnList {
				returns = append(returns, convertFlight(flight, group.Price, true, true))
			}
		}
	}
	return outward, returns
}

func convertFlight(src xmlFlight, groupPrice price, hasReturn bool, isReturn bool) Flight {
	segments := make([]Segment, 0, len(src.SegmentList))
	for _, segment := range src.SegmentList {
		segments = append(segments, Segment{
			Origin:          locationCode(segment.Origin),
			Destination:     locationCode(segment.Destination),
			DepartureTime:   parseTFTime(segment.DepartureRaw),
			ArrivalTime:     parseTFTime(segment.ArrivalRaw),
			DurationMinutes: segment.Duration,
			FlightNumber:    strings.TrimSpace(segment.FlightID.Code),
			TravelClass:     strings.TrimSpace(segment.TravelClass),
		})
	}

	flightPrice := src.Price
	if flightPrice.Amount == 0 && groupPrice.Amount != 0 {
		flightPrice.Currency = groupPrice.Currency
		if !hasReturn || !isReturn {
			flightPrice.Amount = groupPrice.Amount
		}
	}
	if flightPrice.Currency == "" {
		flightPrice.Currency = groupPrice.Currency
	}

	origin := locationCode(src.Origin)
	destination := locationCode(src.Destination)
	if origin == "" && len(segments) > 0 {
		origin = segments[0].Origin
	}
	if destination == "" && len(segments) > 0 {
		destination = segments[len(segments)-1].Destination
	}

	departureTime := parseTFTime(src.DepartureRaw)
	arrivalTime := parseTFTime(src.ReturnRaw)
	if departureTime.IsZero() && len(segments) > 0 {
		departureTime = segments[0].DepartureTime
	}
	if arrivalTime.IsZero() && len(segments) > 0 {
		arrivalTime = segments[len(segments)-1].ArrivalTime
	}

	duration := src.Duration
	if duration == 0 {
		for _, segment := range segments {
			duration += segment.DurationMinutes
		}
	}

	return Flight{
		ID:                 strings.TrimSpace(src.ID),
		Origin:             origin,
		Destination:        destination,
		DepartureTime:      departureTime,
		ArrivalTime:        arrivalTime,
		DurationMinutes:    duration,
		Price:              flightPrice.Amount,
		Currency:           strings.TrimSpace(flightPrice.Currency),
		Segments:           segments,
		MinimalTravelClass: minimalTravelClass(segments),
	}
}

func locationCode(loc xmlLocation) string {
	return strings.TrimSpace(loc.Code)
}

func minimalTravelClass(segments []Segment) string {
	for _, segment := range segments {
		if strings.TrimSpace(segment.TravelClass) != "" {
			return strings.TrimSpace(segment.TravelClass)
		}
	}
	return ""
}

func routingComplete(resp checkRoutingResponse) bool {
	if parseBoolish(resp.Summary.Complete) {
		return true
	}
	if len(resp.RouterList) == 0 {
		return false
	}
	for _, router := range resp.RouterList {
		if parseBoolish(router.Complete) || parseBoolish(router.SearchComplete) {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(router.Status))
		if status == "complete" || status == "completed" || status == "done" {
			continue
		}
		return false
	}
	return true
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
