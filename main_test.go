package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/pavlo-v-chernykh/keystore-go/v4"
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
func TestConvertJKStoPEM(t *testing.T) {
	// Create a temp JKS file for testing
	ks := keystore.New()
	password := "password"

	// Create a self-signed certificate
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	certBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)

	ks.SetTrustedCertificateEntry("test", keystore.TrustedCertificateEntry{
		CreationTime: time.Now(),
		Certificate: keystore.Certificate{
			Type:    "X509",
			Content: certBytes,
		},
	})

	ks.SetPrivateKeyEntry("test-key", keystore.PrivateKeyEntry{
		CreationTime: time.Now(),
		PrivateKey:   priv.D.Bytes(),
		CertificateChain: []keystore.Certificate{
			{
				Type:    "X509",
				Content: certBytes,
			},
		},
	}, []byte(password))

	jksFile, _ := os.CreateTemp("", "test-*.jks")
	defer os.Remove(jksFile.Name())
	ks.Store(jksFile, []byte(password))
	jksFile.Close()

	// Test conversion
	pemPath, err := convertJKStoPEM(jksFile.Name(), password)
	if err != nil {
		t.Fatalf("convertJKStoPEM failed: %v", err)
	}
	defer os.Remove(pemPath)

	// Verify PEM content
	pemData, _ := os.ReadFile(pemPath)
	// Should have at least two certificates now
	count := 0
	rest := pemData
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			count++
		}
	}
	if count < 2 {
		t.Errorf("Expected at least 2 certificates in PEM, got %d", count)
	}

	// Test with wrong password
	_, err = convertJKStoPEM(jksFile.Name(), "wrong")
	if err == nil {
		t.Error("Expected error with wrong password, got nil")
	}

	// Test with non-existent file
	_, err = convertJKStoPEM("nonexistent.jks", password)
	if err == nil {
		t.Error("Expected error with non-existent file, got nil")
	}
}
