package main

import (
	"testing"
)

func TestParseJAASConfig(t *testing.T) {
	tests := []struct {
		name     string
		jaas     string
		wantUser string
		wantPass string
	}{
		{
			name:     "Standard JAAS",
			jaas:     `org.apache.kafka.common.security.plain.PlainLoginModule required username="user" password="password";`,
			wantUser: "user",
			wantPass: "password",
		},
		{
			name:     "Empty JAAS",
			jaas:     "",
			wantUser: "",
			wantPass: "",
		},
		{
			name:     "Incomplete JAAS",
			jaas:     `username="onlyuser"`,
			wantUser: "onlyuser",
			wantPass: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUser, gotPass := parseJAASConfig(tt.jaas)
			if gotUser != tt.wantUser {
				t.Errorf("parseJAASConfig() gotUser = %v, want %v", gotUser, tt.wantUser)
			}
			if gotPass != tt.wantPass {
				t.Errorf("parseJAASConfig() gotPass = %v, want %v", gotPass, tt.wantPass)
			}
		})
	}
}
