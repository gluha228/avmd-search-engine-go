package travelfusion

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestBuildStartRoutingXML(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	returnDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	payload, err := buildStartRoutingXML("xml-login", "login", 60, SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "LON",
		DepartureDate:        departure,
		ReturnDate:           &returnDate,
		AdultCount:           2,
	})
	if err != nil {
		t.Fatalf("buildStartRoutingXML returned error: %v", err)
	}

	xmlBody := string(payload)
	requiredParts := []string{
		"<XmlLoginId>xml-login</XmlLoginId>",
		"<LoginId>login</LoginId>",
		"<Mode>plane</Mode>",
		"<Descriptor>KIV</Descriptor>",
		"<Descriptor>LON</Descriptor>",
		"<Radius>0</Radius>",
		"<DateOfSearch>01/07/2026-00:00</DateOfSearch>",
		"<DiscardBefore>28/06/2026-00:00</DiscardBefore>",
		"<DiscardAfter>05/07/2026-00:00</DiscardAfter>",
		"<DateOfSearch>10/07/2026-00:00</DateOfSearch>",
		"<DiscardBefore>07/07/2026-00:00</DiscardBefore>",
		"<DiscardAfter>14/07/2026-00:00</DiscardAfter>",
		"<MaxChanges>10</MaxChanges>",
		"<MaxHops>11</MaxHops>",
		"<Timeout>60</Timeout>",
		"<IncrementalResults>true</IncrementalResults>",
	}
	for _, part := range requiredParts {
		if !strings.Contains(xmlBody, part) {
			t.Fatalf("expected XML to contain %q, got %s", part, xmlBody)
		}
	}
	if strings.Count(xmlBody, "<Traveller>") != 2 {
		t.Fatalf("expected 2 adult travellers, got XML %s", xmlBody)
	}
}

func TestBuildStartRoutingXMLAddsChildrenAndInfants(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	payload, err := buildStartRoutingXML("xml-login", "login", 60, SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "LON",
		DepartureDate:        departure,
		AdultCount:           1,
		ChildCount:           1,
		InfantCount:          1,
	})
	if err != nil {
		t.Fatalf("buildStartRoutingXML returned error: %v", err)
	}

	xmlBody := string(payload)
	for _, part := range []string{"<Age>30</Age>", "<Age>7</Age>", "<Age>0</Age>"} {
		if !strings.Contains(xmlBody, part) {
			t.Fatalf("expected XML to contain %q, got %s", part, xmlBody)
		}
	}
}

func TestBuildGetCurrenciesXML(t *testing.T) {
	payload, err := buildGetCurrenciesXML("xml-login", "login")
	if err != nil {
		t.Fatalf("buildGetCurrenciesXML returned error: %v", err)
	}

	xmlBody := string(payload)
	for _, part := range []string{
		"<GetCurrencies>",
		"<XmlLoginId>xml-login</XmlLoginId>",
		"<LoginId>login</LoginId>",
	} {
		if !strings.Contains(xmlBody, part) {
			t.Fatalf("expected XML to contain %q, got %s", part, xmlBody)
		}
	}
}

func TestMapCurrencies(t *testing.T) {
	body := []byte(`<CommandList>
  <GetCurrencies>
    <CurrencyList>
      <Currency>
        <Name>Euro</Name>
        <Code>eur</Code>
        <UsdRate>0.9</UsdRate>
      </Currency>
    </CurrencyList>
  </GetCurrencies>
</CommandList>`)

	var resp commandListGetCurrenciesResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		t.Fatalf("xml.Unmarshal returned error: %v", err)
	}
	currencies := mapCurrencies(resp.GetCurrencies)
	if currencies["EUR"].Name != "Euro" || currencies["EUR"].USDRate != 0.9 {
		t.Fatalf("unexpected currencies: %+v", currencies)
	}
}

func TestBuildGetBranchSupplierListXML(t *testing.T) {
	payload, err := buildGetBranchSupplierListXML("xml-login", "login")
	if err != nil {
		t.Fatalf("buildGetBranchSupplierListXML returned error: %v", err)
	}
	xmlBody := string(payload)
	for _, part := range []string{
		"<GetBranchSupplierList>",
		"<XmlLoginId>xml-login</XmlLoginId>",
		"<LoginId>login</LoginId>",
	} {
		if !strings.Contains(xmlBody, part) {
			t.Fatalf("expected XML to contain %q, got %s", part, xmlBody)
		}
	}
}

func TestBuildListSupplierRoutesXML(t *testing.T) {
	payload, err := buildListSupplierRoutesXML("xml-login", "login", "easyjet", false)
	if err != nil {
		t.Fatalf("buildListSupplierRoutesXML returned error: %v", err)
	}
	xmlBody := string(payload)
	for _, part := range []string{
		"<ListSupplierRoutes>",
		"<Supplier>easyjet</Supplier>",
		"<OneWayOnlyAirportRoutes>false</OneWayOnlyAirportRoutes>",
	} {
		if !strings.Contains(xmlBody, part) {
			t.Fatalf("expected XML to contain %q, got %s", part, xmlBody)
		}
	}
}

func TestParseRouteCodes(t *testing.T) {
	routes := parseRouteCodes("madjfk BAD LONPAR MADJFK\nOTPTLV")
	expected := []string{"MADJFK", "LONPAR", "OTPTLV"}
	if len(routes) != len(expected) {
		t.Fatalf("expected %d routes, got %+v", len(expected), routes)
	}
	for i := range expected {
		if routes[i] != expected[i] {
			t.Fatalf("expected route %q at %d, got %+v", expected[i], i, routes)
		}
	}
}

func TestUnmarshalGetBranchSupplierListResponse(t *testing.T) {
	body := []byte(`<CommandList>
  <GetBranchSupplierList>
    <BranchSupplierList>
      <Supplier>easyjet</Supplier>
      <Supplier>ryanair</Supplier>
    </BranchSupplierList>
  </GetBranchSupplierList>
</CommandList>`)

	var resp commandListGetBranchSupplierListResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		t.Fatalf("xml.Unmarshal returned error: %v", err)
	}
	if len(resp.GetBranchSupplierList.Suppliers) != 2 || resp.GetBranchSupplierList.Suppliers[0] != "easyjet" {
		t.Fatalf("unexpected suppliers: %+v", resp.GetBranchSupplierList.Suppliers)
	}
}

func TestUnmarshalListSupplierRoutesResponse(t *testing.T) {
	body := []byte(`<CommandList>
  <ListSupplierRoutes>
    <RouteList>
      <AirportRoutes>OTPCLJ CLJOTP</AirportRoutes>
      <CityRoutes>LONPAR</CityRoutes>
    </RouteList>
  </ListSupplierRoutes>
</CommandList>`)

	var resp commandListListSupplierRoutesResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		t.Fatalf("xml.Unmarshal returned error: %v", err)
	}
	if got := parseRouteCodes(resp.ListSupplierRoutes.AirportRoutes); len(got) != 2 || got[0] != "OTPCLJ" {
		t.Fatalf("unexpected airport routes: %+v", got)
	}
	if got := parseRouteCodes(resp.ListSupplierRoutes.CityRoutes); len(got) != 1 || got[0] != "LONPAR" {
		t.Fatalf("unexpected city routes: %+v", got)
	}
}

func TestExtractFlights(t *testing.T) {
	body := []byte(`<CommandList>
  <CheckRouting>
    <RoutingId>RID123</RoutingId>
    <RouterList>
      <Router>
        <Complete>true</Complete>
        <GroupList>
          <Group>
            <Price>
              <Amount>120.50</Amount>
              <Currency>EUR</Currency>
            </Price>
            <OutwardList>
              <Outward>
                <Id>OUT1</Id>
                <Duration>90</Duration>
                <SegmentList>
                  <Segment>
                    <Origin><Code>KIV</Code></Origin>
                    <Destination><Code>OTP</Code></Destination>
                    <DepartDate>01/07/2026-08:00</DepartDate>
                    <ArriveDate>01/07/2026-09:30</ArriveDate>
                    <Duration>90</Duration>
                    <FlightId><Code>TF100</Code></FlightId>
                    <TravelClass>Economy</TravelClass>
                  </Segment>
                </SegmentList>
              </Outward>
            </OutwardList>
          </Group>
        </GroupList>
      </Router>
    </RouterList>
  </CheckRouting>
</CommandList>`)

	var resp commandListCheckRoutingResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		t.Fatalf("xml.Unmarshal returned error: %v", err)
	}
	outward, returns := extractFlights(resp.CheckRouting)
	if len(outward) != 1 {
		t.Fatalf("expected 1 outward flight, got %d", len(outward))
	}
	if len(returns) != 0 {
		t.Fatalf("expected no return flights, got %d", len(returns))
	}
	if outward[0].ID != "OUT1" || outward[0].Origin != "KIV" || outward[0].Destination != "OTP" {
		t.Fatalf("unexpected outward flight: %+v", outward[0])
	}
	if outward[0].Price != 120.50 || outward[0].Currency != "EUR" {
		t.Fatalf("unexpected price: %+v", outward[0])
	}
	if len(outward[0].Segments) != 1 || outward[0].Segments[0].FlightNumber != "TF100" {
		t.Fatalf("unexpected segments: %+v", outward[0].Segments)
	}
}
