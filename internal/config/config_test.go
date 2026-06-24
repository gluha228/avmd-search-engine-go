package config

import "testing"

func TestValidateDoesNotRequireGoogleSheetsConfigWhenDisabled(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidateRequiresGoogleSheetsConfigWhenEnabled(t *testing.T) {
	cfg := &Config{GoogleSheetsContactDetailsEnabled: true}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing Google Sheets config to fail")
	}

	cfg.GoogleSheetsCredentialsFile = "/tmp/credentials.json"
	cfg.GoogleSheetsSpreadsheetID = "spreadsheet-id"
	cfg.GoogleSheetsContactDetailsRange = "Contacts!A:C"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
