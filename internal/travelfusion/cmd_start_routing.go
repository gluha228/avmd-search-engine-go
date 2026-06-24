package travelfusion

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"
)

type commandListStartRouting struct {
	XMLName      xml.Name            `xml:"CommandList"`
	StartRouting startRoutingCommand `xml:"StartRouting"`
}

type startRoutingCommand struct {
	XmlLoginID         string        `xml:"XmlLoginId"`
	LoginID            string        `xml:"LoginId"`
	Mode               string        `xml:"Mode"`
	Origin             location      `xml:"Origin"`
	Destination        location      `xml:"Destination"`
	OutwardDates       routingDates  `xml:"OutwardDates"`
	ReturnDates        *routingDates `xml:"ReturnDates,omitempty"`
	MaxChanges         int           `xml:"MaxChanges"`
	MaxHops            int           `xml:"MaxHops"`
	Timeout            int           `xml:"Timeout"`
	TravellerList      travellerList `xml:"TravellerList"`
	IncrementalResults bool          `xml:"IncrementalResults"`
}

type location struct {
	Descriptor string `xml:"Descriptor"`
	Type       string `xml:"Type"`
	Radius     int    `xml:"Radius"`
}

type routingDates struct {
	DateOfSearch     string           `xml:"DateOfSearch"`
	DepartDateFilter departDateFilter `xml:"DepartDateFilter"`
}

type departDateFilter struct {
	DiscardBefore string `xml:"DiscardBefore"`
	DiscardAfter  string `xml:"DiscardAfter"`
}

type travellerList struct {
	Travellers []traveller `xml:"Traveller"`
}

type traveller struct {
	Age int `xml:"Age"`
}

type commandListStartRoutingResponse struct {
	StartRouting startRoutingResponse `xml:"StartRouting"`
}

type startRoutingResponse struct {
	RoutingID  string   `xml:"RoutingId"`
	RouterList []router `xml:"RouterList>Router"`
}

func newStartRoutingCommand(xmlLoginID, loginID string, timeoutSeconds int, req SearchRequest) startRoutingCommand {
	cmd := startRoutingCommand{
		XmlLoginID: xmlLoginID,
		LoginID:    loginID,
		Mode:       "plane",
		Origin: location{
			Descriptor: req.DepartureAirportCode,
			Type:       "airportcode",
			Radius:     0,
		},
		Destination: location{
			Descriptor: req.ArrivalAirportCode,
			Type:       "airportcode",
			Radius:     0,
		},
		OutwardDates:       buildRoutingDates(req.DepartureDate),
		MaxChanges:         10,
		MaxHops:            11,
		Timeout:            timeoutSeconds,
		TravellerList:      travellerList{Travellers: buildTravellers(req)},
		IncrementalResults: true,
	}
	if req.ReturnDate != nil {
		returnDates := buildRoutingDates(*req.ReturnDate)
		cmd.ReturnDates = &returnDates
	}
	return cmd
}

func (c *Client) startRouting(ctx context.Context, cmd startRoutingCommand) (startRoutingResponse, error) {
	payload, err := xml.Marshal(commandListStartRouting{StartRouting: cmd})
	if err != nil {
		return startRoutingResponse{}, fmt.Errorf("build start routing request: %w", err)
	}
	body, err := c.postXML(ctx, "StartRouting", payload)
	if err != nil {
		return startRoutingResponse{}, fmt.Errorf("start routing: %w", err)
	}
	var resp commandListStartRoutingResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return startRoutingResponse{}, fmt.Errorf("parse start routing response: %w", err)
	}
	return resp.StartRouting, nil
}

func buildTravellers(req SearchRequest) []traveller {
	travellers := make([]traveller, 0, req.AdultCount+req.ChildCount+req.InfantCount)
	travellers = appendTravellers(travellers, req.AdultCount, 30)
	travellers = appendTravellers(travellers, req.ChildCount, 7)
	travellers = appendTravellers(travellers, req.InfantCount, 0)
	return travellers
}

func appendTravellers(travellers []traveller, count int, age int) []traveller {
	for range count {
		travellers = append(travellers, traveller{Age: age})
	}
	return travellers
}

func buildRoutingDates(date time.Time) routingDates {
	return routingDates{
		DateOfSearch: formatTFTime(date),
		DepartDateFilter: departDateFilter{
			DiscardBefore: formatTFTime(date.AddDate(0, 0, -3)),
			DiscardAfter:  formatTFTime(date.AddDate(0, 0, 4)),
		},
	}
}
