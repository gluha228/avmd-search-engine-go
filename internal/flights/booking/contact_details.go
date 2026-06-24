package booking

import (
	"avmd-search-engine-go/internal/flights/session"
	"context"
	"fmt"
	"strings"
)

func (s *Service) SaveContactDetails(ctx context.Context, searchID string, details ContactData) error {
	if s.sessionStore == nil {
		return fmt.Errorf("%w: session store is not configured", ErrNotFound)
	}
	searchID = strings.TrimSpace(searchID)
	if searchID == "" {
		return fmt.Errorf("%w: searchId is required", ErrInvalidRequest)
	}
	if err := validateContactDetails(details); err != nil {
		return err
	}

	searchSession, err := s.sessionStore.Get(ctx, searchID)
	if err != nil {
		return fmt.Errorf("%w: search session expired or not found for ID: %s", ErrNotFound, searchID)
	}
	normalized := normalizeContactDetails(details)
	searchSession.ContactDetails = &normalized
	if err := s.sessionStore.Save(ctx, searchID, *searchSession); err != nil {
		return fmt.Errorf("update contact details in session: %w", err)
	}

	if s.contactDetailsSink != nil {
		if err := s.contactDetailsSink.AppendContactDetails(ctx, normalized, s.now()); err != nil {
			return fmt.Errorf("append contact details: %w", err)
		}
	}
	return nil
}

func validateContactDetails(details session.ContactData) error {
	if strings.TrimSpace(details.Email) == "" {
		return fmt.Errorf("%w: email is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(details.Phone.InternationalCode) == "" {
		return fmt.Errorf("%w: phone.international_code is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(details.Phone.Number) == "" {
		return fmt.Errorf("%w: phone.number is required", ErrInvalidRequest)
	}
	return nil
}

func normalizeContactDetails(details session.ContactData) session.ContactData {
	return session.ContactData{
		Email: strings.TrimSpace(details.Email),
		Phone: session.Phone{
			InternationalCode: strings.TrimSpace(details.Phone.InternationalCode),
			Number:            strings.TrimSpace(details.Phone.Number),
		},
	}
}
