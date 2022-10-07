package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/RHEnVision/provisioning-backend/internal/ptr"
	"github.com/ilyakaznacheev/cleanenv"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

type proxy struct {
	URL string `env:"URL" env-default:"" env-description:"proxy URL (dev only)"`
}

var config struct {
	App struct {
		Port           int    `env:"PORT" env-default:"8000" env-description:"HTTP port of the API service"`
		Compression    bool   `env:"COMPRESSION" env-default:"false" env-description:"HTTP payload compression"`
		InstancePrefix string `env:"INSTANCE_PREFIX" env-default:"" env-description:"prefix for all VMs names"`
		Cache          struct {
			Expiration      time.Duration `env:"EXPIRATION" env-default:"1h" env-description:"in-memory cache expiration"`
			CleanupInterval time.Duration `env:"CLEANUP_INTERVAL" env-default:"5m" env-description:"in-memory expiration interval"`
			AppTypeId       bool          `env:"APP_TYPE_ID" env-default:"true" env-description:"sources app_type_id cache"`
			Account         bool          `env:"ACCOUNT" env-default:"true" env-description:"account DB ID caching"`
		} `env-prefix:"CACHE_"`
	} `env-prefix:"APP_"`
	Database struct {
		Host        string        `env:"HOST" env-default:"localhost" env-description:"main database hostname"`
		Port        uint16        `env:"PORT" env-default:"5432" env-description:"main database port"`
		Name        string        `env:"NAME" env-default:"provisioning" env-description:"main database name"`
		User        string        `env:"USER" env-default:"postgres" env-description:"main database username"`
		Password    string        `env:"PASSWORD" env-default:"" env-description:"main database password"`
		SeedScript  string        `env:"SEED_SCRIPT" env-default:"" env-description:"database seed script (dev only)"`
		MinConn     int32         `env:"MIN_CONN" env-default:"2" env-description:"connection pool minimum size"`
		MaxConn     int32         `env:"MAX_CONN" env-default:"50" env-description:"connection pool maximum size"`
		MaxIdleTime time.Duration `env:"MAX_IDLE_TIME" env-default:"15m" env-description:"connection pool idle time"`
		MaxLifetime time.Duration `env:"MAX_LIFETIME" env-default:"2h" env-description:"connection pool total lifetime"`
		LogLevel    string        `env:"LOG_LEVEL" env-default:"info" env-description:"logging level of database logs"`
	} `env-prefix:"DATABASE_"`
	Logging struct {
		// TODO make this string
		Level    int  `env:"LEVEL" env-default:"1" env-description:"main application logger level"`
		Stdout   bool `env:"STDOUT" env-default:"true" env-description:"main logger standard output"`
		MaxField int  `env:"MAX_FIELD" env-default:"0" env-description:"main logger maximum field length (dev only)"`
	} `env-prefix:"LOGGING_"`
	Telemetry struct {
		Enabled bool `env:"ENABLED" env-default:"false" env-description:"open telemetry collecting"`
		Jaeger  struct {
			Enabled  bool   `env:"ENABLED" env-default:"false" env-description:"open telemetry jaeger exporter"`
			Endpoint string `env:"ENDPOINT" env-default:"http://localhost:14268/api/traces" env-description:"jaeger endpoint"`
		} `env-prefix:"JAEGER_"`
		Logger struct {
			Enabled bool `env:"ENABLED" env-default:"false" env-description:"open telemetry logger output (dev only)"`
		} `env-prefix:"LOGGER_"`
	} `env-prefix:"TELEMETRY_"`
	Cloudwatch struct {
		Enabled bool   `env:"ENABLED" env-default:"false" env-description:"cloudwatch logging exporter"`
		Region  string `env:"REGION" env-default:"" env-description:"cloudwatch logging AWS region"`
		Key     string `env:"KEY" env-default:"" env-description:"cloudwatch logging key"`
		Secret  string `env:"SECRET" env-default:"" env-description:"cloudwatch logging secret"`
		Session string `env:"SESSION" env-default:"" env-description:"cloudwatch logging session"`
		Group   string `env:"GROUP" env-default:"" env-description:"cloudwatch logging group"`
		Stream  string `env:"STREAM" env-default:"" env-description:"cloudwatch logging stream"`
	} `env-prefix:"CLOUDWATCH_"`
	AWS struct {
		Key           string `env:"KEY" env-default:"" env-description:"AWS service account key"`
		Secret        string `env:"SECRET" env-default:"" env-description:"AWS service account secret"`
		Session       string `env:"SESSION" env-default:"" env-description:"AWS service account session"`
		DefaultRegion string `env:"DEFAULT_REGION" env-default:"us-east-1" env-description:"AWS region when not provided"`
		Logging       bool   `env:"LOGGING" env-default:"false" env-description:"AWS service account logging (verbose)"`
	} `env-prefix:"AWS_"`
	Azure struct {
		TenantID       string `env:"TENANT_ID" env-default:"" env-description:"Azure service account tenant id"`
		SubscriptionID string `env:"SUBSCRIPTION_ID" env-default:"" env-description:"Azure service account subscription id"`
		ClientID       string `env:"CLIENT_ID" env-default:"" env-description:"Azure service account client id"`
		ClientSecret   string `env:"CLIENT_SECRET" env-default:"" env-description:"Azure service account client secret"`
		DefaultRegion  string `env:"DEFAULT_REGION" env-default:"eastus" env-description:"Azure region when not provided"`
	} `env-prefix:"AZURE_"`
	GCP struct {
		JSON        string `env:"JSON" env-default:"e30K" env-description:"GCP service account credentials (base64 encoded)"`
		DefaultZone string `env:"DEFAULT_ZONE" env-default:"us-east1" env-description:"GCP region when not provided"`
	} `env-prefix:"GCP_"`
	Prometheus struct {
		Port int    `env:"PORT" env-default:"9000" env-description:"prometheus HTTP port"`
		Path string `env:"PATH" env-default:"/metrics" env-description:"prometheus metrics path"`
	} `env-prefix:"PROMETHEUS_"`
	RestEndpoints struct {
		ImageBuilder struct {
			URL      string `env:"URL" env-default:"" env-description:"image builder URL"`
			Username string `env:"USERNAME" env-default:"" env-description:"image builder credentials (dev only)"`
			Password string `env:"PASSWORD" env-default:"" env-description:"image builder credentials (dev only)"`
			Proxy    proxy  `env-prefix:"PROXY_" env-description:"image builder HTTP proxy (dev only)"`
		} `env-prefix:"IMAGE_BUILDER_"`
		Sources struct {
			URL      string `env:"URL" env-default:"" env-description:"sources URL"`
			Username string `env:"USERNAME" env-default:"" env-description:"sources credentials (dev only)"`
			Password string `env:"PASSWORD" env-default:"" env-description:"sources credentials (dev only)"`
			Proxy    proxy  `env-prefix:"PROXY_" env-description:"sources HTTP proxy (dev only)"`
		} `env-prefix:"SOURCES_"`
		TraceData bool `env:"TRACE_DATA" env-default:"true" env-description:"open telemetry HTTP context pass and trace"`
	} `env-prefix:"REST_ENDPOINTS_"`
	Worker struct {
		Queue       string `env:"QUEUE" env-default:"memory" env-description:"job worker implementation (memory, sqs, postgres)"`
		Concurrency int    `env:"CONCURRENCY" env-default:"50" env-description:"number of goroutines handling jobs"`
		// TODO time.Duration
		HeartbeatSec int `env:"HEARTBEAT_SEC" env-default:"30" env-description:"heartbeat interval"`
		MaxBeats     int `env:"MAX_BEATS" env-default:"10" env-description:"maximum amount of heartbeats allowed"`
	} `env-prefix:"WORKER_"`
}

// Config shortcuts
var (
	Application   = &config.App
	Database      = &config.Database
	Prometheus    = &config.Prometheus
	Logging       = &config.Logging
	Telemetry     = &config.Telemetry
	Cloudwatch    = &config.Cloudwatch
	AWS           = &config.AWS
	Azure         = &config.Azure
	GCP           = &config.GCP
	RestEndpoints = &config.RestEndpoints
	ImageBuilder  = &config.RestEndpoints.ImageBuilder
	Sources       = &config.RestEndpoints.Sources
	Worker        = &config.Worker
)

// Errors
var (
	validateMissingSecretError      = errors.New("config error: Cloudwatch enabled but Region and Key and Secret are not provided")
	validateGroupStreamError        = errors.New("config error: Cloudwatch enabled but Group or Stream is blank")
	validateInvalidEnvironmentError = errors.New("config error: Environment must be production or development")
)

// Initialize loads configuration from provided .env files, the first existing file wins.
func Initialize(configFiles ...string) {
	var loaded bool
	for _, configFile := range configFiles {
		if _, err := os.Stat(configFile); err == nil {
			// if config file exists, load it (also loads environmental variables)
			err := cleanenv.ReadConfig(configFile, &config)
			if err != nil {
				panic(err)
			}
			loaded = true
		}
	}

	if !loaded {
		// otherwise use only environmental variables instead
		err := cleanenv.ReadEnv(&config)
		if err != nil {
			panic(err)
		}
	}

	// override some values when Clowder is present
	if clowder.IsClowderEnabled() {
		cfg := clowder.LoadedConfig

		// database
		config.Database.Host = cfg.Database.Hostname
		config.Database.Port = uint16(cfg.Database.Port)
		config.Database.User = cfg.Database.Username
		config.Database.Password = cfg.Database.Password
		config.Database.Name = cfg.Database.Name

		// prometheus
		config.Prometheus.Port = cfg.MetricsPort
		config.Prometheus.Path = cfg.MetricsPath

		// HTTP proxies are not allowed in clowder environment
		config.RestEndpoints.Sources.Proxy.URL = ""
		config.RestEndpoints.ImageBuilder.Proxy.URL = ""

		// endpoints configuration
		if endpoint, ok := clowder.DependencyEndpoints["sources-api"]["svc"]; ok {
			config.RestEndpoints.Sources.URL = fmt.Sprintf("http://%s:%d/api/sources/v3.1", endpoint.Hostname, endpoint.Port)
		}
	}

	// validate configuration
	if err := validate(); err != nil {
		panic(err)
	}
}

func HelpText() (string, error) {
	text, err := cleanenv.GetDescription(&config, ptr.To(""))
	if err != nil {
		return "", fmt.Errorf("cannot generate help text: %w", err)
	}
	return text, nil
}