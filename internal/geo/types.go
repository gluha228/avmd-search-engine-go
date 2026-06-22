package geo

type Country struct {
	ID     int64
	NameRo string
	NameRu string
	NameEn string
	ISO3   string
	ISO2   string
}

type CountryRequest struct {
	NameRo string
	NameRu string
	NameEn string
	ISO3   string
	ISO2   string
}

type City struct {
	ID         int64
	CountryID  int64
	NameRo     string
	NameRu     string
	NameEn     string
	IsCapital  bool
	Population *int64
	Timezone   *string
}

type CityRequest struct {
	CountryID  int64
	NameRo     string
	NameRu     string
	NameEn     string
	IsCapital  *bool
	Population *int64
	Timezone   *string
}

type Airport struct {
	ID       int64
	CityID   int64
	IATACode *string
	ICAOCode *string
	Lat      *float64
	Lon      *float64
}

type AirportRequest struct {
	CityID   int64
	IATACode *string
	ICAOCode *string
	Lat      *float64
	Lon      *float64
}

type CityDropdownRequest struct {
	Search            string
	OriginAirportCode *string
	Limit             int
	Locale            string
}

type CityDropdown struct {
	ID          int64
	Name        string
	CountryName string
	AirportCode string
}
