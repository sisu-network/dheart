package config

import "time"

var defaultJobTimeout = 3 * time.Minute

type TimeoutConfig struct {
	PresignJobTimeout      time.Duration
	KeygenJobTimeout       time.Duration
	SigningJobTimeout      time.Duration
	MonitorMessageTimeout  time.Duration
	SelectionLeaderTimeout time.Duration
	SelectionMemberTimeout time.Duration
}

func NewDefaultTimeoutConfig() TimeoutConfig {
	selectionLeaderTimeout := time.Second * 15

	return TimeoutConfig{
		KeygenJobTimeout:       defaultJobTimeout,
		SigningJobTimeout:      defaultJobTimeout,
		PresignJobTimeout:      defaultJobTimeout,
		MonitorMessageTimeout:  time.Second * 15,
		SelectionLeaderTimeout: selectionLeaderTimeout + time.Second*15,
		SelectionMemberTimeout: selectionLeaderTimeout,
	}
}
