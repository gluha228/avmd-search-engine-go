package travelfusion

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
)

type commandListCheckRouting struct {
	XMLName      xml.Name            `xml:"CommandList"`
	CheckRouting checkRoutingCommand `xml:"CheckRouting"`
}

type checkRoutingCommand struct {
	XmlLoginID string `xml:"XmlLoginId"`
	LoginID    string `xml:"LoginId"`
	RoutingID  string `xml:"RoutingId"`
}

type commandListCheckRoutingResponse struct {
	CheckRouting checkRoutingResponse `xml:"CheckRouting"`
}

type checkRoutingResponse struct {
	RoutingID  string         `xml:"RoutingId"`
	RouterList []router       `xml:"RouterList>Router"`
	Summary    routingSummary `xml:"Summary"`
}

type routingSummary struct {
	Complete string `xml:"Complete"`
}

type router struct {
	Complete       string  `xml:"Complete"`
	SearchComplete string  `xml:"SearchComplete"`
	Status         string  `xml:"Status"`
	GroupList      []group `xml:"GroupList>Group"`
	Groups         []group `xml:"Group"`
}

type group struct {
	ID          string      `xml:"Id"`
	Price       price       `xml:"Price"`
	OutwardList []xmlFlight `xml:"OutwardList>Outward"`
	ReturnList  []xmlFlight `xml:"ReturnList>Return"`
}

type xmlFlight struct {
	ID           string       `xml:"Id"`
	Origin       xmlLocation  `xml:"Origin"`
	Destination  xmlLocation  `xml:"Destination"`
	Price        price        `xml:"Price"`
	Duration     int          `xml:"Duration"`
	SegmentList  []xmlSegment `xml:"SegmentList>Segment"`
	DepartureRaw string       `xml:"OutwardDate"`
	ReturnRaw    string       `xml:"ReturnDate"`
}

type xmlSegment struct {
	Origin       xmlLocation `xml:"Origin"`
	Destination  xmlLocation `xml:"Destination"`
	DepartureRaw string      `xml:"DepartDate"`
	ArrivalRaw   string      `xml:"ArriveDate"`
	Duration     int         `xml:"Duration"`
	FlightID     flightID    `xml:"FlightId"`
	TravelClass  travelClass `xml:"TravelClass"`
}

type travelClass struct {
	Value   string `xml:",chardata"`
	TfClass string `xml:"TfClass"`
}

type xmlLocation struct {
	Code string `xml:"Code"`
}

type flightID struct {
	Code string `xml:"Code"`
}

type price struct {
	Amount   float64 `xml:"Amount"`
	Currency string  `xml:"Currency"`
}

func newCheckRoutingCommand(xmlLoginID, loginID, routingID string) checkRoutingCommand {
	return checkRoutingCommand{
		XmlLoginID: xmlLoginID,
		LoginID:    loginID,
		RoutingID:  routingID,
	}
}

func (c *Client) checkRouting(ctx context.Context, cmd checkRoutingCommand) (checkRoutingResponse, error) {
	payload, err := xml.Marshal(commandListCheckRouting{CheckRouting: cmd})
	if err != nil {
		return checkRoutingResponse{}, fmt.Errorf("build check routing request: %w", err)
	}
	body, err := c.postXML(ctx, "CheckRouting", payload)
	if err != nil {
		return checkRoutingResponse{}, fmt.Errorf("check routing: %w", err)
	}
	var resp commandListCheckRoutingResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return checkRoutingResponse{}, fmt.Errorf("parse check routing response: %w", err)
	}
	return resp.CheckRouting, nil
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
			TravelClass:     segment.TravelClass.value(),
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

func (c travelClass) value() string {
	if strings.TrimSpace(c.TfClass) != "" {
		return strings.TrimSpace(c.TfClass)
	}
	return strings.TrimSpace(c.Value)
}

func routingComplete(resp checkRoutingResponse) bool {
	if parseBoolish(resp.Summary.Complete) {
		return true
	}
	return !routingNeedsPolling(resp.RouterList)
}

func routingNeedsPolling(routers []router) bool {
	for _, router := range routers {
		if !routerComplete(router) {
			return true
		}
	}
	return false
}

func completedRouterCount(routers []router) int {
	count := 0
	for _, router := range routers {
		if routerComplete(router) {
			count++
		}
	}
	return count
}

func routerComplete(router router) bool {
	if parseBoolish(router.Complete) || parseBoolish(router.SearchComplete) {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(router.Status))
	return status == "complete" || status == "completed" || status == "done"
}
