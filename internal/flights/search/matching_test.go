package search

import (
	"avmd-search-engine-go/internal/travelfusion"
	"testing"
	"time"
)

func TestBuildOffersPairsOnlyFlightsFromSameGroup(t *testing.T) {
	departure := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	returnDate := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)

	offers := buildOffers(
		[]travelfusion.Flight{
			testFlight("OUT1", "G1", "KIV", "OTP", departure, 100),
			testFlight("OUT2", "G2", "KIV", "OTP", departure, 200),
		},
		[]travelfusion.Flight{
			testFlight("RET1", "G1", "OTP", "KIV", returnDate, 50),
			testFlight("RET2", "G3", "OTP", "KIV", returnDate, 10),
		},
		true,
	)

	if len(offers) != 1 {
		t.Fatalf("expected only one same-group offer, got %d: %+v", len(offers), offers)
	}
	if offers[0].OfferID != "TF-OUT1-RET1" || offers[0].Price != 150 {
		t.Fatalf("unexpected same-group offer: %+v", offers[0])
	}
}

func TestBuildOffersSkipsRoundTripFlightsWithoutGroup(t *testing.T) {
	departure := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	returnDate := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)

	offers := buildOffers(
		[]travelfusion.Flight{testFlight("OUT1", "", "KIV", "OTP", departure, 100)},
		[]travelfusion.Flight{testFlight("RET1", "", "OTP", "KIV", returnDate, 50)},
		true,
	)

	if len(offers) != 0 {
		t.Fatalf("expected no round-trip offers without group IDs, got %+v", offers)
	}
}

func testFlight(id, groupID, origin, destination string, departure time.Time, price float64) travelfusion.Flight {
	arrival := departure.Add(90 * time.Minute)
	return travelfusion.Flight{
		ID:            id,
		GroupID:       groupID,
		Origin:        origin,
		Destination:   destination,
		DepartureTime: departure,
		ArrivalTime:   arrival,
		Price:         price,
		Currency:      "EUR",
		Segments: []travelfusion.Segment{
			{
				Origin:          origin,
				Destination:     destination,
				DepartureTime:   departure,
				ArrivalTime:     arrival,
				DurationMinutes: 90,
			},
		},
	}
}
