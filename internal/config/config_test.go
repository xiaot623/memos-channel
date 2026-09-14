package config

import "testing"

func TestServerURL(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{addr: "dns:memos:5230", want: "http://memos:5230"},
		{addr: "localhost:5230", want: "http://localhost:5230"},
		{addr: "https://keep.example", want: "https://keep.example"},
		{addr: "http://memos:5230", want: "http://memos:5230"},
	}
	for _, tt := range tests {
		cfg := &Config{ServerAddr: tt.addr}
		if got := cfg.ServerURL(); got != tt.want {
			t.Errorf("ServerURL(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}

func TestHTTPSOrigin(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "https://keep.xiaot.tech", want: "https://keep.xiaot.tech"},
		{in: "https://keep.xiaot.tech/", want: "https://keep.xiaot.tech"},
		{in: "  https://keep.xiaot.tech/  ", want: "https://keep.xiaot.tech"},
		{in: "http://keep.xiaot.tech", want: ""},
		{in: "http://memos:5230", want: ""},
		{in: "dns:memos:5230", want: ""},
		{in: "https://", want: ""},
		{in: "", want: ""},
	}
	for _, tt := range tests {
		if got := HTTPSOrigin(tt.in); got != tt.want {
			t.Errorf("HTTPSOrigin(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
