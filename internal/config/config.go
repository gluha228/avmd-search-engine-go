package config

import "github.com/caarlos0/env"

type Config struct {
	Port                   int    `env:"API_PORT" envDefault:"8080"`
	DefaultCurrencyCode    string `env:"DEFAULT_CURRENCY_CODE" envDefault:"EUR"`
	LoggingLevel           string `env:"LOG_LEVEL" envDefault:"INFO"`
	LoggingAddSource       bool   `env:"LOG_ADD_SOURCE" envDefault:"false"`
	UseJsonLogs            bool   `env:"LOG_USE_JSON" envDefault:"false"`
	DatabaseURL            string `env:"DB_URL" envDefault:"postgres://postgres:postgres@localhost:5432/avion?sslmode=disable"`
	DatabaseAutoMigrate    bool   `env:"DB_AUTO_MIGRATE" envDefault:"true"`
	RedisAddr              string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword          string `env:"REDIS_PASSWORD"`
	RedisDB                int    `env:"REDIS_DB" envDefault:"0"`
	RedisSessionTTLHours   int    `env:"REDIS_SESSION_TTL_HOURS" envDefault:"24"`
	RedisCalendarTTLHours  int    `env:"REDIS_CALENDAR_TTL_HOURS" envDefault:"720"`
	TFBaseURL              string `env:"TF_BASE_URL" envDefault:"http://api.travelfusion.com"`
	TFXmlLoginID           string `env:"TF_XML_LOGIN_ID"`
	TFLoginID              string `env:"TF_LOGIN_ID"`
	TFTimeoutSeconds       int    `env:"TF_TIMEOUT_SECONDS" envDefault:"60"`
	TFPollingAttempts      int    `env:"TF_POLLING_ATTEMPTS" envDefault:"10"`
	TFPollingDelaySeconds  int    `env:"TF_POLLING_DELAY_SECONDS" envDefault:"2"`
	TFCurrenciesUpdateCron string `env:"TF_CURRENCIES_UPDATE_CRON" envDefault:"0 0 3 * * ?"`
	TFCurrenciesUpdateTime string `env:"TF_CURRENCIES_UPDATE_TIME" envDefault:"03:00"`
}

func GetFromEnv() (*Config, error) {
	config := &Config{}
	err := env.Parse(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
