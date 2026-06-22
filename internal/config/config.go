package config

import "github.com/caarlos0/env"

type Config struct {
	Port                  int    `env:"API_PORT" envDefault:"8080"`
	LoggingLevel          string `env:"LOG_LEVEL" envDefault:"INFO"`
	LoggingAddSource      bool   `env:"LOG_ADD_SOURCE" envDefault:"false"`
	UseJsonLogs           bool   `env:"LOG_USE_JSON" envDefault:"false"`
	TFBaseURL             string `env:"TF_BASE_URL" envDefault:"http://api.travelfusion.com"`
	TFXmlLoginID          string `env:"TF_XML_LOGIN_ID"`
	TFLoginID             string `env:"TF_LOGIN_ID"`
	TFTimeoutSeconds      int    `env:"TF_TIMEOUT_SECONDS" envDefault:"60"`
	TFPollingAttempts     int    `env:"TF_POLLING_ATTEMPTS" envDefault:"10"`
	TFPollingDelaySeconds int    `env:"TF_POLLING_DELAY_SECONDS" envDefault:"2"`
}

func GetFromEnv() (*Config, error) {
	config := &Config{}
	err := env.Parse(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
