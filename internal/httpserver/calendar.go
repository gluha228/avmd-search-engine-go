package httpserver

import (
	api "avmd-search-engine-go/api/gen"
	"avmd-search-engine-go/internal/calendar"
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *HttpServer) GetCalendar(
	ctx context.Context,
	request api.GetCalendarRequestObject,
) (api.GetCalendarResponseObject, error) {
	serviceReq := calendar.Request{
		DepartureAirportCode: request.Params.DepartureAirportCode,
		ArrivalAirportCode:   request.Params.ArrivalAirportCode,
		DateFrom:             request.Params.DateFrom.Time,
		DateTo:               request.Params.DateTo.Time,
	}

	serviceResp, err := s.calendarService.GetCalendar(ctx, serviceReq)
	if errors.Is(err, calendar.ErrInvalidRequest) {
		return api.GetCalendar400JSONResponse{Message: err.Error()}, nil
	}
	if err != nil {
		return api.GetCalendar500JSONResponse{Message: err.Error()}, nil
	}

	days := make([]api.FlightDay, len(serviceResp.Calendar))
	for i := range serviceResp.Calendar {
		date, err := time.Parse(time.DateOnly, serviceResp.Calendar[i].Date)
		if err != nil {
			return api.GetCalendar500JSONResponse{Message: err.Error()}, nil
		}
		days[i] = api.FlightDay{
			Date:         openapi_types.Date{Time: date},
			Price:        serviceResp.Calendar[i].Price,
			CurrencyCode: serviceResp.Calendar[i].CurrencyCode,
		}
	}
	return api.GetCalendar200JSONResponse{Calendar: days}, nil
}
