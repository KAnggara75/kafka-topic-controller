package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_KAFKA_1", "val1")
	defer os.Unsetenv("TEST_KAFKA_1")

	tests := []struct {
		name     string
		keys     []string
		expected string
	}{
		{
			name:     "First key exists",
			keys:     []string{"TEST_KAFKA_1", "TEST_KAFKA_2"},
			expected: "val1",
		},
		{
			name:     "Second key exists",
			keys:     []string{"TEST_KAFKA_NONEXISTENT", "TEST_KAFKA_1"},
			expected: "val1",
		},
		{
			name:     "No keys exist",
			keys:     []string{"NONEXISTENT1", "NONEXISTENT2"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getEnv(tt.keys...); got != tt.expected {
				t.Errorf("getEnv() = %v, want %v", got, tt.expected)
			}
		})
	}
}

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
