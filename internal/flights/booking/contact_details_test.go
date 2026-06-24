package booking

import (
	"avmd-search-engine-go/internal/flights/session"
	"context"
	"testing"
	"time"
)

type fakeSessionStore struct {
	session session.FlightSearchSession
	err     error
}

func (f *fakeSessionStore) Save(_ context.Context, _ string, session session.FlightSearchSession) error {
	f.session = session
	return f.err
}

func (f *fakeSessionStore) Get(_ context.Context, _ string) (*session.FlightSearchSession, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &f.session, nil
}

type fakeContactDetailsSink struct {
	details   session.ContactData
	createdAt time.Time
	called    bool
	err       error
}

func (f *fakeContactDetailsSink) AppendContactDetails(_ context.Context, details session.ContactData, createdAt time.Time) error {
	f.called = true
	f.details = details
	f.createdAt = createdAt
	return f.err
}

func TestSaveContactDetailsSavesSessionAndAppendsSink(t *testing.T) {
	store := &fakeSessionStore{}
	sink := &fakeContactDetailsSink{}
	service := NewService(nil, store, nil, nil, "", nil)
	service.SetContactDetailsSink(sink)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	err := service.SaveContactDetails(context.Background(), "search-1", session.ContactData{
		Email: " user@example.com ",
		Phone: session.Phone{InternationalCode: " +373 ", Number: " 69123456 "},
	})
	if err != nil {
		t.Fatalf("SaveContactDetails returned error: %v", err)
	}
	if store.session.ContactDetails == nil {
		t.Fatal("expected contact details to be saved in session")
	}
	if store.session.ContactDetails.Email != "user@example.com" ||
		store.session.ContactDetails.Phone.InternationalCode != "+373" ||
		store.session.ContactDetails.Phone.Number != "69123456" {
		t.Fatalf("unexpected saved contact details: %+v", store.session.ContactDetails)
	}
	if !sink.called || sink.createdAt != now || sink.details.Email != "user@example.com" {
		t.Fatalf("expected sink to receive normalized contact details, got %+v", sink)
	}
}

func TestSaveContactDetailsValidatesRequiredFields(t *testing.T) {
	service := NewService(nil, &fakeSessionStore{}, nil, nil, "", nil)

	err := service.SaveContactDetails(context.Background(), "search-1", session.ContactData{
		Phone: session.Phone{InternationalCode: "+373", Number: "69123456"},
	})
	if err == nil {
		t.Fatal("expected missing email to fail")
	}
}
