package travelfusion

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBuildStartRoutingXML(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	returnDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	payload, err := marshalStartRoutingCommand(newStartRoutingCommand("xml-login", "login", 60, SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "LON",
		DepartureDate:        departure,
		ReturnDate:           &returnDate,
		AdultCount:           2,
	}))
	if err != nil {
		t.Fatalf("marshalStartRoutingCommand returned error: %v", err)
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

	payload, err := marshalStartRoutingCommand(newStartRoutingCommand("xml-login", "login", 60, SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "LON",
		DepartureDate:        departure,
		AdultCount:           1,
		ChildCount:           1,
		InfantCount:          1,
	}))
	if err != nil {
		t.Fatalf("marshalStartRoutingCommand returned error: %v", err)
	}

	xmlBody := string(payload)
	for _, part := range []string{"<Age>30</Age>", "<Age>7</Age>", "<Age>0</Age>"} {
		if !strings.Contains(xmlBody, part) {
			t.Fatalf("expected XML to contain %q, got %s", part, xmlBody)
		}
	}
}

func marshalStartRoutingCommand(cmd startRoutingCommand) ([]byte, error) {
	return xml.Marshal(commandListStartRouting{StartRouting: cmd})
}

func marshalGetCurrenciesCommand(cmd getCurrenciesCommand) ([]byte, error) {
	return xml.Marshal(commandListGetCurrencies{GetCurrencies: cmd})
}

func marshalProcessDetailsCommand(cmd processDetailsCommand) ([]byte, error) {
	return xml.Marshal(commandListProcessDetails{ProcessDetails: cmd})
}

func marshalGetBranchSupplierListCommand(cmd getBranchSupplierListCommand) ([]byte, error) {
	return xml.Marshal(commandListGetBranchSupplierList{GetBranchSupplierList: cmd})
}

func marshalListSupplierRoutesCommand(cmd listSupplierRoutesCommand) ([]byte, error) {
	return xml.Marshal(commandListListSupplierRoutes{ListSupplierRoutes: cmd})
}

func TestBuildGetCurrenciesXML(t *testing.T) {
	payload, err := marshalGetCurrenciesCommand(newGetCurrenciesCommand("xml-login", "login"))
	if err != nil {
		t.Fatalf("marshalGetCurrenciesCommand returned error: %v", err)
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

func TestBuildProcessDetailsXML(t *testing.T) {
	payload, err := marshalProcessDetailsCommand(newProcessDetailsCommand("xml-login", "login", ProcessDetailsRequest{
		RoutingID: "RID",
		OutwardID: "OUT1",
		ReturnID:  "RET1",
	}))
	if err != nil {
		t.Fatalf("marshalProcessDetailsCommand returned error: %v", err)
	}

	xmlBody := string(payload)
	for _, part := range []string{
		"<ProcessDetails>",
		"<XmlLoginId>xml-login</XmlLoginId>",
		"<LoginId>login</LoginId>",
		"<RoutingId>RID</RoutingId>",
		"<OutwardId>OUT1</OutwardId>",
		"<ReturnId>RET1</ReturnId>",
	} {
		if !strings.Contains(xmlBody, part) {
			t.Fatalf("expected XML to contain %q, got %s", part, xmlBody)
		}
	}
}

func TestMapProcessDetailsRequiredParameters(t *testing.T) {
	body := []byte(`<CommandList>
  <ProcessDetails>
    <RoutingId>RID</RoutingId>
    <Router>
      <RequiredParameterList>
        <RequiredParameter>
          <Name>PassportNumber</Name>
          <Type>text</Type>
          <DisplayText>Passport number</DisplayText>
          <PerPassenger>true</PerPassenger>
          <IsOptional>false</IsOptional>
          <IsSometimesRequired/>
        </RequiredParameter>
      </RequiredParameterList>
    </Router>
  </ProcessDetails>
</CommandList>`)

	var resp commandListProcessDetailsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		t.Fatalf("xml.Unmarshal returned error: %v", err)
	}
	result := mapProcessDetails(resp.ProcessDetails)
	if result.RoutingID != "RID" || len(result.RequiredParameters) != 1 {
		t.Fatalf("unexpected process details result: %+v", result)
	}
	param := result.RequiredParameters[0]
	if param.Name != "PassportNumber" || param.Type != "TEXT" || param.PerPassenger == nil || !*param.PerPassenger || !param.IsSometimesRequired {
		t.Fatalf("unexpected required parameter: %+v", param)
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
	payload, err := marshalGetBranchSupplierListCommand(newGetBranchSupplierListCommand("xml-login", "login"))
	if err != nil {
		t.Fatalf("marshalGetBranchSupplierListCommand returned error: %v", err)
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
	payload, err := marshalListSupplierRoutesCommand(newListSupplierRoutesCommand("xml-login", "login", "easyjet", false))
	if err != nil {
		t.Fatalf("marshalListSupplierRoutesCommand returned error: %v", err)
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
                    <TfOperator><Name>Bangkok Airways</Name><Code>PG</Code></TfOperator>
                    <FlightId><Code>TF100</Code></FlightId>
                    <TravelClass>
                      <TfClass>Economy With Restrictions</TfClass>
                      <SupplierClass>PROMO</SupplierClass>
                    </TravelClass>
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
	if outward[0].Segments[0].Operator.Name != "Bangkok Airways" || outward[0].Segments[0].Operator.Code != "PG" {
		t.Fatalf("unexpected operator: %+v", outward[0].Segments[0].Operator)
	}
	if outward[0].Segments[0].TravelClass != "Economy With Restrictions" || outward[0].MinimalTravelClass != "Economy With Restrictions" {
		t.Fatalf("unexpected travel class: %+v", outward[0])
	}
}

func TestExtractFlightsReadsPassengerPrices(t *testing.T) {
	body := []byte(`<CommandList>
  <CheckRouting>
    <RouterList>
      <Router>
        <GroupList>
          <Group>
            <OutwardList>
              <Outward>
                <Id>OUT1</Id>
                <Origin><Code>KIV</Code></Origin>
                <Destination><Code>OTP</Code></Destination>
                <Price>
                  <Amount>250</Amount>
                  <Currency>EUR</Currency>
                  <PassengerPriceList>
                    <PassengerPrice><Amount>100</Amount><Age>30</Age></PassengerPrice>
                    <PassengerPrice><Amount>90</Amount><Age>30</Age></PassengerPrice>
                    <PassengerPrice><Amount>50</Amount><Age>7</Age></PassengerPrice>
                    <PassengerPrice><Amount>10</Amount><Age>0</Age></PassengerPrice>
                  </PassengerPriceList>
                </Price>
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
	outward, _ := extractFlights(resp.CheckRouting)
	if len(outward) != 1 {
		t.Fatalf("expected 1 outward flight, got %d", len(outward))
	}
	if !slices.Equal(outward[0].PassengerPrices.Adults, []float64{100, 90}) {
		t.Fatalf("unexpected adult prices: %+v", outward[0].PassengerPrices)
	}
	if !slices.Equal(outward[0].PassengerPrices.Children, []float64{50}) {
		t.Fatalf("unexpected child prices: %+v", outward[0].PassengerPrices)
	}
	if !slices.Equal(outward[0].PassengerPrices.Infants, []float64{10}) {
		t.Fatalf("unexpected infant prices: %+v", outward[0].PassengerPrices)
	}
}

func TestExtractFlightsReadsTextTravelClass(t *testing.T) {
	body := []byte(`<CommandList>
  <CheckRouting>
    <RouterList>
      <Router>
        <GroupList>
          <Group>
            <Price><Amount>100</Amount><Currency>EUR</Currency></Price>
            <OutwardList>
              <Outward>
                <Id>OUT1</Id>
                <SegmentList>
                  <Segment>
                    <Origin><Code>KIV</Code></Origin>
                    <Destination><Code>OTP</Code></Destination>
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
	outward, _ := extractFlights(resp.CheckRouting)
	if len(outward) != 1 || len(outward[0].Segments) != 1 || outward[0].Segments[0].TravelClass != "Economy" {
		t.Fatalf("unexpected travel class: %+v", outward)
	}
}

func TestRoutingNeedsPollingUsesRouterCompleteFlags(t *testing.T) {
	if !routingNeedsPolling([]router{{Complete: "false"}, {Complete: "true"}}) {
		t.Fatal("expected polling while any router is incomplete")
	}
	if routingNeedsPolling([]router{{Complete: "true"}, {SearchComplete: "true"}}) {
		t.Fatal("expected no polling when all routers are complete")
	}
	if routingNeedsPolling([]router{{Status: "completed"}}) {
		t.Fatal("expected completed status to stop polling")
	}
}

func TestRoutingCompleteTreatsEmptyRouterListAsComplete(t *testing.T) {
	if !routingComplete(checkRoutingResponse{}) {
		t.Fatal("expected empty router list to be complete because no incomplete routers remain")
	}
}

func TestSearchStreamLogsPollingAttempts(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		requests++
		w.Header().Set("Content-Type", "text/xml")
		if strings.Contains(string(body), "<StartRouting>") {
			_, _ = w.Write([]byte(`<CommandList>
  <StartRouting>
    <RoutingId>RID</RoutingId>
    <RouterList>
      <Router><Complete>false</Complete></Router>
      <Router><Complete>false</Complete></Router>
    </RouterList>
  </StartRouting>
</CommandList>`))
			return
		}
		_, _ = w.Write([]byte(`<CommandList>
  <CheckRouting>
    <RoutingId>RID</RoutingId>
    <RouterList>
      <Router><Complete>true</Complete></Router>
      <Router><Complete>false</Complete></Router>
    </RouterList>
  </CheckRouting>
</CommandList>`))
	}))
	defer server.Close()

	var logBuffer bytes.Buffer
	client := NewClient(Config{
		BaseURL:             server.URL,
		XmlLoginID:          "xml",
		LoginID:             "login",
		TimeoutSeconds:      5,
		PollingAttempts:     3,
		PollingDelaySeconds: 0,
	}, slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug})))

	for update := range client.SearchStream(context.Background(), SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "OTP",
		DepartureDate:        time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		AdultCount:           1,
	}) {
		if update.Err != nil {
			t.Fatalf("unexpected search update error: %v", update.Err)
		}
	}

	if requests != 4 {
		t.Fatalf("expected StartRouting and three CheckRouting requests, got %d", requests)
	}
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "Poll attempt") ||
		!strings.Contains(logOutput, "attempt=1") ||
		!strings.Contains(logOutput, "max_attempts=3") ||
		!strings.Contains(logOutput, "routing_id=RID") {
		t.Fatalf("expected poll attempt debug log, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "travelfusion search routers started") ||
		!strings.Contains(logOutput, "routers_started=2") {
		t.Fatalf("expected started routers debug log, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "TravelFusion search routers completed") ||
		!strings.Contains(logOutput, "completed_routers=1") ||
		!strings.Contains(logOutput, "total_routers=2") {
		t.Fatalf("expected completed routers debug log, got %q", logOutput)
	}
}

func TestSearchStreamDoesNotSleepBeforeFirstCheckRouting(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		requests++
		w.Header().Set("Content-Type", "text/xml")
		if strings.Contains(string(body), "<StartRouting>") {
			_, _ = w.Write([]byte(`<CommandList>
  <StartRouting>
    <RoutingId>RID</RoutingId>
    <RouterList>
      <Router><Complete>false</Complete></Router>
    </RouterList>
  </StartRouting>
</CommandList>`))
			return
		}
		_, _ = w.Write([]byte(`<CommandList>
  <CheckRouting>
    <RoutingId>RID</RoutingId>
    <RouterList>
      <Router><Complete>true</Complete></Router>
    </RouterList>
  </CheckRouting>
</CommandList>`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:             server.URL,
		XmlLoginID:          "xml",
		LoginID:             "login",
		TimeoutSeconds:      5,
		PollingAttempts:     3,
		PollingDelaySeconds: 60,
	}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	for update := range client.SearchStream(ctx, SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "OTP",
		DepartureDate:        time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		AdultCount:           1,
	}) {
		if update.Err != nil {
			t.Fatalf("unexpected search update error: %v", update.Err)
		}
	}

	if requests != 2 {
		t.Fatalf("expected StartRouting and immediate first CheckRouting request, got %d requests", requests)
	}
}
