package search

import (
	"avmd-search-engine-go/internal/flights/session"
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"fmt"
	"strings"
)

type SearchRequest = session.SearchRequest
type SearchResponse = session.SearchResponse
type SearchOffersUpdate = session.SearchOffersUpdate
type Offer = session.Offer
type EnrichedOffer = session.EnrichedOffer

func (s *Service) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	searchID, err := s.CreateSession(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.SearchIntoSession(ctx, searchID, req, nil)
}

func (s *Service) CreateSession(ctx context.Context, req SearchRequest) (string, error) {
	if err := s.Validate(req); err != nil {
		return "", err
	}
	if s.sessionStore == nil {
		return "", nil
	}
	return s.sessionStore.Create(ctx, session.FlightSearchSession{Params: req})
}

func (s *Service) SearchIntoSession(
	ctx context.Context,
	searchID string,
	req SearchRequest,
	onOffers func([]Offer) error,
) (*SearchResponse, error) {
	var response SearchResponse
	seenUpdate := false
	for update := range s.SearchIntoSessionStream(ctx, searchID, req) {
		if update.Err != nil {
			return nil, update.Err
		}
		seenUpdate = true
		response.SearchID = update.SearchID
		response.RoutingID = update.RoutingID
		response.Offers = update.Offers
		if len(update.Offers) > 0 && onOffers != nil {
			if err := onOffers(update.Offers); err != nil {
				return nil, err
			}
		}
	}
	if !seenUpdate {
		response.SearchID = searchID
	}
	return &response, nil
}

func (s *Service) SearchIntoSessionStream(
	ctx context.Context,
	searchID string,
	req SearchRequest,
) <-chan SearchOffersUpdate {
	updates := make(chan SearchOffersUpdate)
	go func() {
		defer close(updates)
		s.searchIntoSessionStream(ctx, searchID, req, updates)
	}()
	return updates
}

func (s *Service) searchIntoSessionStream(
	ctx context.Context,
	searchID string,
	req SearchRequest,
	updates chan<- SearchOffersUpdate,
) {
	if err := s.Validate(req); err != nil {
		sendFlightSearchUpdate(ctx, updates, SearchOffersUpdate{SearchID: searchID, Err: err})
		return
	}

	tfReq := travelfusion.SearchRequest{
		DepartureAirportCode: req.DepartureAirportCode,
		ArrivalAirportCode:   req.ArrivalAirportCode,
		DepartureDate:        req.DepartureDate,
		ReturnDate:           req.ReturnDate,
		AdultCount:           req.AdultCount,
		ChildCount:           req.ChildCount,
		InfantCount:          req.InfantCount,
	}

	var routingID string
	var outwardFlights []travelfusion.Flight
	var returnFlights []travelfusion.Flight
	var offers []Offer
	for tfUpdate := range s.tfClient.SearchStream(ctx, tfReq) {
		if tfUpdate.Err != nil {
			sendFlightSearchUpdate(ctx, updates, SearchOffersUpdate{SearchID: searchID, RoutingID: routingID, Offers: offers, Err: tfUpdate.Err})
			return
		}
		if strings.TrimSpace(tfUpdate.RoutingID) != "" {
			routingID = tfUpdate.RoutingID
		}
		s.cacheCalendarFlights(ctx, req, tfUpdate)
		outwardFlights = mergeTravelfusionFlights(outwardFlights, tfUpdate.OutwardFlights)
		returnFlights = mergeTravelfusionFlights(returnFlights, tfUpdate.ReturnFlights)

		offers = s.mapSearchOffers(req, outwardFlights, returnFlights)
		if s.logger != nil {
			s.logger.Debug("flight search mapped", "routing_id", routingID, "offers", len(offers))
		}

		if err := s.saveSearchSession(ctx, searchID, req, routingID, offers); err != nil {
			sendFlightSearchUpdate(ctx, updates, SearchOffersUpdate{SearchID: searchID, RoutingID: routingID, Offers: offers, Err: err})
			return
		}
		if !sendFlightSearchUpdate(ctx, updates, SearchOffersUpdate{
			SearchID:  searchID,
			RoutingID: routingID,
			Offers:    offers,
		}) {
			return
		}
	}
}

func (s *Service) cacheCalendarFlights(ctx context.Context, req SearchRequest, update travelfusion.SearchUpdate) {
	if s.calendar == nil {
		return
	}
	if len(update.OutwardFlights) > 0 {
		if err := s.calendar.CacheFlights(ctx, req.DepartureAirportCode, req.ArrivalAirportCode, update.OutwardFlights); err != nil && s.logger != nil {
			s.logger.Warn("failed to cache outbound calendar prices", "error", err)
		}
	}
	if req.ReturnDate != nil && len(update.ReturnFlights) > 0 {
		if err := s.calendar.CacheFlights(ctx, req.ArrivalAirportCode, req.DepartureAirportCode, update.ReturnFlights); err != nil && s.logger != nil {
			s.logger.Warn("failed to cache inbound calendar prices", "error", err)
		}
	}
}

func (s *Service) saveSearchSession(ctx context.Context, searchID string, req SearchRequest, routingID string, offers []Offer) error {
	if s.sessionStore == nil || searchID == "" {
		return nil
	}
	err := s.sessionStore.Save(ctx, searchID, session.FlightSearchSession{
		Params:      req,
		TFRoutingID: routingID,
		TFOffers:    offers,
	})
	if err != nil {
		return fmt.Errorf("update flight search session: %w", err)
	}
	return nil
}

func sendFlightSearchUpdate(ctx context.Context, updates chan<- SearchOffersUpdate, update SearchOffersUpdate) bool {
	select {
	case <-ctx.Done():
		return false
	case updates <- update:
		return true
	}
}
