package googlesheets

import (
	"avmd-search-engine-go/internal/flights/session"
	"context"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type ContactDetailsSink struct {
	service       *sheets.Service
	spreadsheetID string
	writeRange    string
}

func NewContactDetailsSink(ctx context.Context, credentialsFile, spreadsheetID, writeRange string) (*ContactDetailsSink, error) {
	service, err := sheets.NewService(ctx, option.WithAuthCredentialsFile(option.ServiceAccount, credentialsFile))
	if err != nil {
		return nil, err
	}
	return &ContactDetailsSink{
		service:       service,
		spreadsheetID: strings.TrimSpace(spreadsheetID),
		writeRange:    strings.TrimSpace(writeRange),
	}, nil
}

func (s *ContactDetailsSink) AppendContactDetails(ctx context.Context, details session.ContactData, createdAt time.Time) error {
	phone := strings.TrimSpace(details.Phone.InternationalCode) + strings.TrimSpace(details.Phone.Number)
	values := &sheets.ValueRange{
		Values: [][]interface{}{
			{
				createdAt.Format(time.RFC3339),
				phone,
				strings.TrimSpace(details.Email),
			},
		},
	}
	_, err := s.service.Spreadsheets.Values.Append(s.spreadsheetID, s.writeRange, values).
		ValueInputOption("RAW").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	return err
}
