package flights

import "time"

type SearchRequest struct {
	DepartureAirportCode                string
	ArrivalAirportCode                  string
	DepartureDate                       time.Time
	ReturnDate                          *time.Time
	AdultCount                          int
	ChildCount                          int
	InfantCount                         int
	MinPrice                            *float64
	MaxPrice                            *float64
	MinSegments                         *int
	MaxSegments                         *int
	MinTotalDurationMinutes             *int
	MaxTotalDurationMinutes             *int
	MinIndividualSegmentDurationMinutes *int
	MaxIndividualSegmentDurationMinutes *int
	MinLayoverMinutes                   *int
	MaxLayoverMinutes                   *int
	DepartureOutboundFrom               *time.Time
	DepartureOutboundTo                 *time.Time
	ArrivalOutboundFrom                 *time.Time
	ArrivalOutboundTo                   *time.Time
	DepartureInboundFrom                *time.Time
	DepartureInboundTo                  *time.Time
	ArrivalInboundFrom                  *time.Time
	ArrivalInboundTo                    *time.Time
}

type SearchResponse struct {
	SearchID  string
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

type FlightSearchSession struct {
	Params      SearchRequest `json:"params"`
	TFRoutingID string        `json:"tf_routing_id"`
	TFOffers    []Offer       `json:"tf_offers"`
}
