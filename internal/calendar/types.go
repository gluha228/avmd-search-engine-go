package calendar

import "time"

type Request struct {
	DepartureAirportCode string
	ArrivalAirportCode   string
	DateFrom             time.Time
	DateTo               time.Time
}

type Response struct {
	Calendar []FlightDay `json:"calendar"`
}

type FlightDay struct {
	Date         string  `json:"date"`
	Price        float64 `json:"price"`
	CurrencyCode string  `json:"currency_code"`
}

type PriceEntry struct {
	Price        float64 `json:"price"`
	CurrencyCode string  `json:"currency_code"`
}
