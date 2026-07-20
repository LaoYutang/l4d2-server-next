package utility

import "testing"

func TestNormalizeLocationIP(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{name: "IPv4", address: "203.0.113.10", want: "203.0.113.10"},
		{name: "IPv4 with port", address: "203.0.113.10:27015", want: "203.0.113.10"},
		{name: "IPv6", address: "2001:db8::10", want: "2001:db8::10"},
		{name: "IPv6 with port", address: "[2001:db8::10]:27015", want: "2001:db8::10"},
		{name: "bracketed IPv6", address: "[2001:db8::10]", want: "2001:db8::10"},
		{name: "localhost", address: "localhost", want: "localhost"},
		{name: "invalid", address: "not-an-ip", want: "not-an-ip"},
		{name: "surrounding whitespace", address: "  203.0.113.10:27015  ", want: "203.0.113.10"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeLocationIP(test.address); got != test.want {
				t.Fatalf("normalizeLocationIP(%q) = %q, want %q", test.address, got, test.want)
			}
		})
	}
}

func TestGetLocationHandlesLocalAndInvalidAddressesWithoutService(t *testing.T) {
	if got := GetLocation("127.0.0.1"); got != "Localhost" {
		t.Fatalf("GetLocation(localhost) = %q, want Localhost", got)
	}
	if got := GetLocation("::1"); got != "Localhost" {
		t.Fatalf("GetLocation(IPv6 localhost) = %q, want Localhost", got)
	}
	if got := GetLocation("[::1]:27015"); got != "Localhost" {
		t.Fatalf("GetLocation(IPv6 localhost with port) = %q, want Localhost", got)
	}
	if got := GetLocation("not-an-ip"); got != "" {
		t.Fatalf("GetLocation(invalid IP) = %q, want empty string", got)
	}
}
