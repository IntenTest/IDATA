package server

import "time"

type Config struct {
	AgentToken            string
	AdminToken            string
	DeviceCredentialsFile string
	EnrollmentAutoApprove bool
	BrowserPairingEnabled bool
	DeviceSessionTTL      time.Duration
	PairingRequestTTL     time.Duration
	DefaultTimeout        time.Duration
	MaxCommandTimeout     time.Duration
}
