package main

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// validAgentPath permits a deployment prefix but never navigation or escaped
// separators. It is shared by launch parsing and native-process handoff.
func validAgentPath(path string) bool {
	if !strings.HasSuffix(path, "/ws/agent") || strings.ContainsAny(path, "\\%?#") {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", c)) {
				return false
			}
		}
	}
	return strings.HasPrefix(path, "/")
}

// A full URL is authoritative. Bare hosts inherit configuration only when they
// refer to the same host; another host never inherits a previous server's TLS.
func serverURLFromInput(input, previous string) (string, error) {
	input = strings.TrimSpace(input)
	if !strings.Contains(input, "://") {
		if validServerHost(input) {
			return serverURLFromEndpoint(input, serverPortForHost(input, previous), previous)
		}
		host, port, err := net.SplitHostPort(input)
		if err != nil {
			return "", errors.New("enter a server URL, hostname, or IP address")
		}
		return serverURLFromEndpoint(host, port, previous)
	}
	parsed, err := url.Parse(input)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawPath != "" {
		return "", errors.New("invalid server URL")
	}
	switch parsed.Scheme {
	case "http", "https":
		if parsed.Scheme == "https" {
			parsed.Scheme = "wss"
		} else {
			parsed.Scheme = "ws"
		}
		parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/ws/agent"
	case "ws", "wss":
		if parsed.Path == "" || parsed.Path == "/" {
			parsed.Path = "/ws/agent"
		}
	default:
		return "", errors.New("unsupported server URL scheme")
	}
	if parsed.Port() == "" {
		port := "80"
		if parsed.Scheme == "wss" {
			port = "443"
		}
		parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
	}
	result := parsed.String()
	if err := validateLaunchServerURL(result); err != nil {
		return "", err
	}
	return result, nil
}
