package googlesheets

import (
	"context"
	"testing"

	"github.com/go-redis/redismock/v9"
)

func TestRedisConfigStoreReturnsDisabledWhenFlagMissing(t *testing.T) {
	db, mock := redismock.NewClientMock()
	store := NewRedisConfigStore(db)
	mock.ExpectGet(contactDetailsEnabledKey).RedisNil()

	cfg, err := store.ContactDetailsConfig(context.Background())
	if err != nil {
		t.Fatalf("ContactDetailsConfig returned error: %v", err)
	}
	if cfg.Enabled {
		t.Fatalf("expected disabled config, got %+v", cfg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations were not met: %v", err)
	}
}

func TestRedisConfigStoreValidatesEnabledConfig(t *testing.T) {
	db, mock := redismock.NewClientMock()
	store := NewRedisConfigStore(db)
	mock.ExpectGet(contactDetailsEnabledKey).SetVal("true")
	mock.ExpectMGet(contactDetailsCredentialsJSONKey, contactDetailsSpreadsheetIDKey, contactDetailsRangeKey).SetVal([]interface{}{"{}", "", "Contacts!A:C"})

	if _, err := store.ContactDetailsConfig(context.Background()); err == nil {
		t.Fatal("expected missing spreadsheet id to fail")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations were not met: %v", err)
	}
}

func TestRedisConfigStoreReadsEnabledConfig(t *testing.T) {
	db, mock := redismock.NewClientMock()
	store := NewRedisConfigStore(db)
	mock.ExpectGet(contactDetailsEnabledKey).SetVal("true")
	mock.ExpectMGet(contactDetailsCredentialsJSONKey, contactDetailsSpreadsheetIDKey, contactDetailsRangeKey).SetVal([]interface{}{"{}", "spreadsheet-id", "Contacts!A:C"})

	cfg, err := store.ContactDetailsConfig(context.Background())
	if err != nil {
		t.Fatalf("ContactDetailsConfig returned error: %v", err)
	}
	if !cfg.Enabled || string(cfg.CredentialsJSON) != "{}" || cfg.SpreadsheetID != "spreadsheet-id" || cfg.Range != "Contacts!A:C" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations were not met: %v", err)
	}
}
