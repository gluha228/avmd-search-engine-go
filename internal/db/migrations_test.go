package db

import "testing"

func TestCityValuesParsesNullableFields(t *testing.T) {
	values, err := cityValues([]string{"1", "2", "Chisinau", "Chisinau", "Kishinev", "false", "", ""})
	if err != nil {
		t.Fatalf("cityValues returned error: %v", err)
	}
	if values[0] != int64(1) || values[1] != int64(2) || values[5] != false {
		t.Fatalf("unexpected fixed values: %+v", values)
	}
	if population, ok := values[6].(*int64); !ok || population != nil {
		t.Fatalf("expected nullable population, got %+v", values[6])
	}
	if timezone, ok := values[7].(*string); !ok || timezone != nil {
		t.Fatalf("expected nullable population/timezone, got %+v", values)
	}
}

func TestAirportValuesParsesCoordinates(t *testing.T) {
	values, err := airportValues([]string{"1", "2", "kiv", "lukk", "46.9277", "28.9309"})
	if err != nil {
		t.Fatalf("airportValues returned error: %v", err)
	}
	if values[0] != int64(1) || values[1] != int64(2) {
		t.Fatalf("unexpected ids: %+v", values)
	}
	if values[2] == nil || values[3] == nil || values[4] == nil || values[5] == nil {
		t.Fatalf("expected non-null airport values: %+v", values)
	}
}
