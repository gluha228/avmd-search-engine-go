package flights

import "time"

type SearchRequest struct {
	DepartureAirportCode string
	ArrivalAirportCode   string
	DepartureDate        time.Time
	ReturnDate           *time.Time
	AdultCount           int
}

type SearchResponse struct {
	RoutingID string
	Offers    []Offer
}

type Offer struct {
	OfferID        string
	OutboundFlight Flight
	InboundFlight  *Flight
	CurrencyCode   string
	Price          float64
}

type Flight struct {
	DepartureAirportCode string
	ArrivalAirportCode   string
	SeatsAvailable       int
	Price                float64
	Segments             []Segment
}

type Segment struct {
	SegmentID            int
	DepartureAirportCode string
	ArrivalAirportCode   string
	DepartureTime        *time.Time
	ArrivalTime          *time.Time
	DurationMinutes      int
	FlightNumber         string
	TravelClass          string
}
