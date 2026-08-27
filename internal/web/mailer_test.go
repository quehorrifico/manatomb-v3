package web

import "testing"

func TestPlaintextSMTPIsLimitedToLoopbackDevelopmentHosts(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"localhost", "127.0.0.1", "::1", "[::1]"} {
		if !allowPlaintextSMTPHost(host) {
			t.Errorf("allowPlaintextSMTPHost(%q) = false", host)
		}
	}
	for _, host := range []string{"smtp.example.com", "10.0.0.8", "mail.internal"} {
		if allowPlaintextSMTPHost(host) {
			t.Errorf("allowPlaintextSMTPHost(%q) = true", host)
		}
	}
}
