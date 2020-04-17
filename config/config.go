package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

var (
	version   string
	buildDate string
)

// Config contains the configuration parameters of this app
type Config struct {
	DatabaseHost       string        `envconfig:"DATABASE_HOST" default:"localhost"`
	DatabasePort       uint          `envconfig:"DATABASE_PORT" default:"5432"`
	DatabaseUser       string        `envconfig:"DATABASE_USER" default:"postgres"`
	DatabasePassword   string        `envconfig:"DATABASE_PASSWORD" default:"postgres"`
	DatabaseName       string        `envconfig:"DATABASE_NAME" default:"ferrum"`
	HTTPAPIPort        uint          `envconfig:"HTTP_API_PORT" default:"80"`
	HTTPRequestTimeout time.Duration `envconfig:"HTTP_REQUEST_TIMEOUT" default:"3s"`
	HTTPMaxPOSTSize    int64         `envconfig:"HTTP_MAX_POST_SIZE" default:"1048576"` // 1MiB
	HTTPJWTSigningKey  string        `envconfig:"HTTP_JWT_SIGNING_KEY" default:"deadbeef"`
	HTTPJWTVClaimName  string        `envconfig:"HTTP_JWT_CLAIM_NAME" default:"ferrum"`
	HTTPJWTExpiration  time.Duration `envconfig:"HTTP_JWT_EXPIRATION" default:"1h"`
	LogLevelRaw        string        `envconfig:"LOG_LEVEL" default:"info"`
	LogLevel           log.Level
	Version            string
	BuildDate          string
}

// Load reads the configuration parameters from environment variables
func Load() (Config, error) {
	var c Config
	err := envconfig.Process("ferrum", &c)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse configuration env vars: %v", err)
	}

	c.LogLevel, err = log.ParseLevel(c.LogLevelRaw)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse log level: %v", err)
	}

	c.Version = version
	c.BuildDate = buildDate

	return c, nil
}
