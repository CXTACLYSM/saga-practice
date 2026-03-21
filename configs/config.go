package configs

import (
	"time"

	"github.com/CXTACLYSM/saga-practice/configs/app"
	"github.com/CXTACLYSM/saga-practice/configs/database/persistence/postgres"
	"github.com/CXTACLYSM/saga-practice/configs/messaging/rabbitmq"
	"github.com/spf13/viper"
)

type Config struct {
	App      app.Config
	Postgres postgres.Config
	RabbitMQ rabbitmq.Config
}

type HttpSrv struct {
}

func NewConfig() *Config {
	viper.AutomaticEnv()

	return &Config{
		App: app.Config{
			Http: app.Http{
				Port:              viper.GetInt("APP_HTTP_PORT"),
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       10 * time.Second,
				WriteTimeout:      35 * time.Second,
				IdleTimeout:       60 * time.Second,
				MaxHeaderBytes:    1 << 20,
			},
		},
		Postgres: postgres.Config{
			Host:     viper.GetString("POSTGRES_HOST"),
			Port:     viper.GetInt("POSTGRES_PORT"),
			User:     viper.GetString("POSTGRES_USER"),
			Password: viper.GetString("POSTGRES_PASSWORD"),
			Db:       viper.GetString("POSTGRES_DB"),
		},
		RabbitMQ: rabbitmq.Config{
			Host:     viper.GetString("RABBITMQ_HOST"),
			Port:     viper.GetInt("RABBITMQ_PORT"),
			Username: viper.GetString("RABBITMQ_USER"),
			Password: viper.GetString("RABBITMQ_PASSWORD"),
		},
	}
}
