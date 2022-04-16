package config

import "time"

var defaultJobTimeout = 10 * time.Minute

type TimeoutConfig struct {
	PresignJobTimeout     time.Duration
	KeygenJobTimeout      time.Duration
	SigningJobTimeout     time.Duration
	MonitorMessageTimeout time.Duration
	PreworkWaitTimeout    time.Duration
}

func NewDefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		KeygenJobTimeout:      defaultJobTimeout,
		SigningJobTimeout:     defaultJobTimeout,
		PresignJobTimeout:     defaultJobTimeout,
		MonitorMessageTimeout: time.Second * 15,
		PreworkWaitTimeout:    time.Second * 15,
	}
}
