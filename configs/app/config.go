package app

import "time"

type Config struct {
	Http Http
}

type Http struct {
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}
