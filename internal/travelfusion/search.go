package travelfusion

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SearchRequest struct {
	DepartureAirportCode string
	ArrivalAirportCode   string
	DepartureDate        time.Time
	ReturnDate           *time.Time
	AdultCount           int
	ChildCount           int
	InfantCount          int
}

type SearchResult struct {
	RoutingID      string
	OutwardFlights []Flight
	ReturnFlights  []Flight
}

type SearchUpdate struct {
	RoutingID      string
	OutwardFlights []Flight
	ReturnFlights  []Flight
	Err            error
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

	startResp, err := c.startRouting(ctx, newStartRoutingCommand(c.xmlLoginID, c.loginID, c.timeoutSeconds, req))
	if err != nil {
		sendSearchError(ctx, updates, err)
		return
	}
	if strings.TrimSpace(startResp.RoutingID) == "" {
		sendSearchError(ctx, updates, fmt.Errorf("travelfusion start routing returned empty routing id"))
		return
	}
	routingID := startResp.RoutingID
	if c.logger != nil {
		c.logger.Debug(
			"travelfusion search routers started",
			"routing_id", routingID,
			"routers_started", len(startResp.RouterList),
		)
	}
	if !sendSearchUpdate(ctx, updates, SearchUpdate{RoutingID: routingID}) {
		return
	}
	if !routingNeedsPolling(startResp.RouterList) {
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

		checkResp, err := c.checkRouting(ctx, newCheckRoutingCommand(c.xmlLoginID, c.loginID, routingID))
		if err != nil {
			sendSearchError(ctx, updates, err)
			return
		}
		if c.logger != nil {
			c.logger.Debug(
				"TravelFusion search routers completed",
				"attempt", attempt+1,
				"max_attempts", c.pollingAttempts,
				"routing_id", routingID,
				"completed_routers", completedRouterCount(checkResp.RouterList),
				"total_routers", len(checkResp.RouterList),
			)
		}
		outward, returns := extractFlights(checkResp)
		if len(outward) > 0 || len(returns) > 0 {
			if !sendSearchUpdate(ctx, updates, SearchUpdate{
				RoutingID:      routingID,
				OutwardFlights: outward,
				ReturnFlights:  returns,
			}) {
				return
			}
		}

		if routingComplete(checkResp) {
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
