package travelfusion

import (
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
	ChildCount           int
	InfantCount          int
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

func formatTFTime(t time.Time) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Format(tfTimeLayout)
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
