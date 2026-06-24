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

type SearchOffersUpdate struct {
	SearchID  string
	RoutingID string
	Offers    []Offer
	Err       error
}

type SelectedOffer struct {
	Offer            Offer
	SearchParams     SearchRequest
	AdditionalFields []AdditionalField
}

type PassengerDataRequest struct {
	SearchID           string
	OfferID            string
	Passengers         []Passenger
	ContactData        ContactData
	SupplierParameters []SupplierParameter
}

type Passenger struct {
	Title                  string
	FirstName              string
	LastName               string
	DateOfBirth            time.Time
	CitizenshipCountryCode string
	SupplierParameters     []SupplierParameter
}

type ContactData struct {
	Email string
	Phone Phone
}

type Phone struct {
	InternationalCode string
	Number            string
}

type SupplierParameter struct {
	ParamName  string
	ParamValue string
}

type PassengerDataResponse struct {
	RoutingID                           string
	TFBookingReference                  string
	FinalAmount                         *float64
	FinalCurrency                       string
	SupplierVisualAuthorisationImageURL string
	SupplierResponses                   []ProcessTermsSupplierResponse
}

type ProcessTermsSupplierResponse struct {
	Name string
	Type string
	Data string
}

type AdditionalField struct {
	Code         string
	Description  string
	InputType    string
	Required     bool
	PerPassenger bool
	Options      []AdditionalFieldOption
}

type AdditionalFieldOption struct {
	Value string
	Label string
	Price *AdditionalFieldOptionPrice
}

type AdditionalFieldOptionPrice struct {
	Amount       float64
	CurrencyCode string
}

type Offer struct {
	OfferID        string
	OutboundFlight Flight
	InboundFlight  *Flight
	CurrencyCode   string
	Price          float64
}

type EnrichedOffer struct {
	OfferID        string
	OutboundFlight EnrichedFlight
	InboundFlight  *EnrichedFlight
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

type EnrichedFlight struct {
	DepartureFlightAirport FlightAirport
	ArrivalFlightAirport   FlightAirport
	SeatsAvailable         int
	Price                  float64
	Segments               []EnrichedSegment
}

type FlightAirport struct {
	Code     string
	CityName string
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

type EnrichedSegment struct {
	SegmentID              int
	DepartureFlightAirport FlightAirport
	ArrivalFlightAirport   FlightAirport
	DepartureTime          *time.Time
	ArrivalTime            *time.Time
	DurationMinutes        int
	FlightNumber           string
	TravelClass            string
}

type FlightSearchSession struct {
	Params               SearchRequest                 `json:"params"`
	TFRoutingID          string                        `json:"tf_routing_id"`
	TFOffers             []Offer                       `json:"tf_offers"`
	TFSeatMapByOfferID   map[string][]SegmentSeatMap   `json:"tf_seat_map_by_offer_id,omitempty"`
	SelectedOfferID      string                        `json:"selected_offer_id,omitempty"`
	TFRequiredParameters []TFRequiredParameterSnapshot `json:"tf_required_parameters,omitempty"`
}

type TFRequiredParameterSnapshot struct {
	Parameter           string `json:"parameter"`
	Value               string `json:"value,omitempty"`
	Type                string `json:"type,omitempty"`
	PerPassenger        *bool  `json:"per_passenger,omitempty"`
	IsOptional          *bool  `json:"is_optional,omitempty"`
	IsSometimesRequired bool   `json:"is_sometimes_required"`
	DisplayText         string `json:"display_text,omitempty"`
}

type SegmentSeatMap struct {
	SegmentID    int
	Origin       string
	Destination  string
	FlightNumber string
	Seats        []SeatDetail
}

type SeatDetail struct {
	Code                       string
	Type                       string
	SeatDescription            *string
	Price                      *float64
	CurrencyCode               *string
	Row                        int
	Col                        int
	IsAvailable                bool
	PersonsWithReducedMobility bool
	NoInfantSeat               bool
}
