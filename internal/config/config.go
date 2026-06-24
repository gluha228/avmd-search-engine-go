package config

import (
	"fmt"
	"strings"

	"github.com/caarlos0/env"
)

type Config struct {
	Port                              int    `env:"API_PORT" envDefault:"8080"`
	DefaultCurrencyCode               string `env:"DEFAULT_CURRENCY_CODE" envDefault:"EUR"`
	LoggingLevel                      string `env:"LOG_LEVEL" envDefault:"INFO"`
	LoggingAddSource                  bool   `env:"LOG_ADD_SOURCE" envDefault:"false"`
	UseJsonLogs                       bool   `env:"LOG_USE_JSON" envDefault:"false"`
	CORSAllowedOrigins                string `env:"CORS_ALLOWED_ORIGINS" envDefault:"http://localhost:3000,http://localhost:5173,http://127.0.0.1:3000,http://127.0.0.1:5173"`
	DatabaseURL                       string `env:"DB_URL" envDefault:"postgres://postgres:postgres@localhost:5432/avion?sslmode=disable"`
	DatabaseAutoMigrate               bool   `env:"DB_AUTO_MIGRATE" envDefault:"true"`
	RedisAddr                         string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword                     string `env:"REDIS_PASSWORD"`
	RedisDB                           int    `env:"REDIS_DB" envDefault:"0"`
	RedisConnectAttempts              int    `env:"REDIS_CONNECT_ATTEMPTS" envDefault:"3"`
	RedisConnectRetryDelaySeconds     int    `env:"REDIS_CONNECT_RETRY_DELAY_SECONDS" envDefault:"1"`
	RedisSessionTTLHours              int    `env:"REDIS_SESSION_TTL_HOURS" envDefault:"24"`
	RedisCalendarTTLHours             int    `env:"REDIS_CALENDAR_TTL_HOURS" envDefault:"720"`
	TFBaseURL                         string `env:"TF_BASE_URL" envDefault:"http://api.travelfusion.com"`
	TFXmlLoginID                      string `env:"TF_XML_LOGIN_ID"`
	TFLoginID                         string `env:"TF_LOGIN_ID"`
	TFTimeoutSeconds                  int    `env:"TF_TIMEOUT_SECONDS" envDefault:"60"`
	TFPollingAttempts                 int    `env:"TF_POLLING_ATTEMPTS" envDefault:"10"`
	TFPollingDelaySeconds             int    `env:"TF_POLLING_DELAY_SECONDS" envDefault:"2"`
	TFOperatorLogoURLPattern          string `env:"TF_OPERATOR_LOGO_URL_PATTERN" envDefault:"https://www.travelfusion.com/images/operators/p%s.gif"`
	TFCurrenciesUpdateCron            string `env:"TF_CURRENCIES_UPDATE_CRON" envDefault:"0 0 3 * * ?"`
	TFCurrenciesUpdateTime            string `env:"TF_CURRENCIES_UPDATE_TIME" envDefault:"03:00"`
	TFRoutesUpdateCron                string `env:"TF_ROUTES_UPDATE_CRON" envDefault:"0 0 4 * * ?"`
	TFRoutesUpdateTime                string `env:"TF_ROUTES_UPDATE_TIME" envDefault:"04:00"`
	GoogleSheetsContactDetailsEnabled bool   `env:"GOOGLE_SHEETS_CONTACT_DETAILS_ENABLED" envDefault:"false"`
	GoogleSheetsCredentialsFile       string `env:"GOOGLE_SHEETS_CREDENTIALS_FILE"`
	GoogleSheetsSpreadsheetID         string `env:"GOOGLE_SHEETS_SPREADSHEET_ID"`
	GoogleSheetsContactDetailsRange   string `env:"GOOGLE_SHEETS_CONTACT_DETAILS_RANGE" envDefault:"Contacts!A:C"`
}

func GetFromEnv() (*Config, error) {
	config := &Config{}
	err := env.Parse(config)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}

func (c *Config) Validate() error {
	if !c.GoogleSheetsContactDetailsEnabled {
		return nil
	}
	if strings.TrimSpace(c.GoogleSheetsCredentialsFile) == "" {
		return fmt.Errorf("GOOGLE_SHEETS_CREDENTIALS_FILE is required when GOOGLE_SHEETS_CONTACT_DETAILS_ENABLED=true")
	}
	if strings.TrimSpace(c.GoogleSheetsSpreadsheetID) == "" {
		return fmt.Errorf("GOOGLE_SHEETS_SPREADSHEET_ID is required when GOOGLE_SHEETS_CONTACT_DETAILS_ENABLED=true")
	}
	if strings.TrimSpace(c.GoogleSheetsContactDetailsRange) == "" {
		return fmt.Errorf("GOOGLE_SHEETS_CONTACT_DETAILS_RANGE is required when GOOGLE_SHEETS_CONTACT_DETAILS_ENABLED=true")
	}
	return nil
}
