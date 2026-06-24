package testsupport

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
)

func SearchUpdateStream(
	ctx context.Context,
	result *travelfusion.SearchResult,
	searchUpdates []travelfusion.SearchUpdate,
	err error,
) <-chan travelfusion.SearchUpdate {
	updates := make(chan travelfusion.SearchUpdate)
	go func() {
		defer close(updates)
		for _, update := range searchUpdatesFrom(result, searchUpdates, err) {
			select {
			case <-ctx.Done():
				return
			case updates <- update:
			}
		}
	}()
	return updates
}

func searchUpdatesFrom(
	result *travelfusion.SearchResult,
	searchUpdates []travelfusion.SearchUpdate,
	err error,
) []travelfusion.SearchUpdate {
	if len(searchUpdates) > 0 {
		return searchUpdates
	}
	if err != nil {
		return []travelfusion.SearchUpdate{{Err: err}}
	}
	if result == nil {
		return nil
	}
	updates := []travelfusion.SearchUpdate{{RoutingID: result.RoutingID}}
	if len(result.OutwardFlights) > 0 || len(result.ReturnFlights) > 0 {
		updates = append(updates, travelfusion.SearchUpdate{
			RoutingID:      result.RoutingID,
			OutwardFlights: result.OutwardFlights,
			ReturnFlights:  result.ReturnFlights,
		})
	}
	return updates
}
