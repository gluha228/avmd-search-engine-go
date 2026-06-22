package travelfusion

import (
	"encoding/xml"
	"strconv"
	"strings"
	"time"
)

const tfTimeLayout = "02/01/2006-15:04"

type SearchRequest struct {
	DepartureAirportCode string
	ArrivalAirportCode   string
	DepartureDate        time.Time
	ReturnDate           *time.Time
	AdultCount           int
}

type SearchResult struct {
	RoutingID      string
	OutwardFlights []Flight
	ReturnFlights  []Flight
}

type Flight struct {
	ID                 string
	Origin             string
	Destination        string
	DepartureTime      time.Time
	ArrivalTime        time.Time
	DurationMinutes    int
	Price              float64
	Currency           string
	Segments           []Segment
	MinimalTravelClass string
}

type Segment struct {
	Origin          string
	Destination     string
	DepartureTime   time.Time
	ArrivalTime     time.Time
	DurationMinutes int
	FlightNumber    string
	TravelClass     string
}

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

type commandListCheckRouting struct {
	XMLName      xml.Name            `xml:"CommandList"`
	CheckRouting checkRoutingCommand `xml:"CheckRouting"`
}

type checkRoutingCommand struct {
	XmlLoginID string `xml:"XmlLoginId"`
	LoginID    string `xml:"LoginId"`
	RoutingID  string `xml:"RoutingId"`
}

type commandListStartRoutingResponse struct {
	StartRouting startRoutingResponse `xml:"StartRouting"`
}

type startRoutingResponse struct {
	RoutingID  string   `xml:"RoutingId"`
	RouterList []router `xml:"RouterList>Router"`
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
	TravelClass  string      `xml:"TravelClass"`
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

func buildStartRoutingXML(xmlLoginID, loginID string, timeoutSeconds int, req SearchRequest) ([]byte, error) {
	travellers := make([]traveller, req.AdultCount)
	for i := range travellers {
		travellers[i] = traveller{Age: 30}
	}

	cmd := commandListStartRouting{
		StartRouting: startRoutingCommand{
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
			TravellerList:      travellerList{Travellers: travellers},
			IncrementalResults: true,
		},
	}
	if req.ReturnDate != nil {
		returnDates := buildRoutingDates(*req.ReturnDate)
		cmd.StartRouting.ReturnDates = &returnDates
	}

	return xml.Marshal(cmd)
}

func buildCheckRoutingXML(xmlLoginID, loginID, routingID string) ([]byte, error) {
	return xml.Marshal(commandListCheckRouting{
		CheckRouting: checkRoutingCommand{
			XmlLoginID: xmlLoginID,
			LoginID:    loginID,
			RoutingID:  routingID,
		},
	})
}

func formatTFTime(t time.Time) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Format(tfTimeLayout)
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

func parseTFTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	t, err := time.ParseInLocation(tfTimeLayout, value, time.Local)
	if err == nil {
		return t
	}
	shortYear, err := time.ParseInLocation("02/01/06-15:04", value, time.Local)
	if err == nil {
		return shortYear
	}
	return time.Time{}
}

func parseBoolish(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return false
	}
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed
	}
	return value == "complete" || value == "completed" || value == "done" || value == "finished"
}
