package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	serverpkg "idata-server/internal/server"
)

const (
	specialListenIP   = "10.90.65.189"
	specialListenPort = "12345"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "idata-server:", err)
		os.Exit(1)
	}
}

func run() error {
	listenAddr := flag.String("listen", envOr("IDATA_LISTEN_ADDR", defaultListenAddr()), "HTTP listen address")
	browserPairing := flag.Bool("browser-pairing", envBool("IDATA_BROWSER_PAIRING", false), "enable the legacy v0.4 visible Windows confirmation API")
	autoApproveEnrollment := flag.Bool("enrollment-auto-approve", envBool("IDATA_ENROLLMENT_AUTO_APPROVE", true), "automatically approve every valid native Client enrollment request")
	deviceSessionTTL := flag.Duration("device-session-ttl", envDuration("IDATA_DEVICE_SESSION_TTL", 8*time.Hour), "browser device-session lifetime")
	pairingRequestTTL := flag.Duration("pairing-request-ttl", envDuration("IDATA_PAIRING_REQUEST_TTL", 2*time.Minute), "Windows pairing confirmation timeout")
	defaultTimeout := flag.Duration("command-timeout", envDuration("IDATA_COMMAND_TIMEOUT", 30*time.Second), "default command timeout")
	maxTimeout := flag.Duration("max-command-timeout", envDuration("IDATA_MAX_COMMAND_TIMEOUT", 5*time.Minute), "maximum command timeout")
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	app, err := serverpkg.New(serverpkg.Config{
		AgentToken:            os.Getenv("IDATA_AGENT_TOKEN"),
		AdminToken:            os.Getenv("IDATA_ADMIN_TOKEN"),
		DeviceCredentialsFile: envOr("IDATA_DEVICE_CREDENTIALS_FILE", "/var/lib/idata/device-credentials.json"),
		EnrollmentAutoApprove: *autoApproveEnrollment,
		BrowserPairingEnabled: *browserPairing,
		DeviceSessionTTL:      *deviceSessionTTL,
		PairingRequestTTL:     *pairingRequestTTL,
		DefaultTimeout:        *defaultTimeout,
		MaxCommandTimeout:     *maxTimeout,
	}, logger)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              *listenAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveError := make(chan error, 1)
	go func() {
		logger.Info("server listening", "address", *listenAddr)
		serveError <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serveError:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func defaultListenAddr() string {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return ":80"
	}
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		values = append(values, address.String())
	}
	return listenAddrForInterfaceAddresses(values)
}

func listenAddrForInterfaceAddresses(addresses []string) string {
	for _, address := range addresses {
		ipText, _, _ := strings.Cut(address, "/")
		if ip := net.ParseIP(ipText); ip != nil && ip.String() == specialListenIP {
			return ":" + specialListenPort
		}
	}
	return ":80"
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
