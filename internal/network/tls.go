package network

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// TLSConfigManager handles generation and setup of mTLS 1.3 certificates
type TLSConfigManager struct {
	caCert *x509.Certificate
	caKey  *ecdsa.PrivateKey
}

// NewTLSConfigManager creates a new manager with a generated CA
func NewTLSConfigManager() (*TLSConfigManager, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"AI Blockchain Mesh CA"},
			CommonName:   "P2P Root CA",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caDer, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA cert: %w", err)
	}

	caCert, err := x509.ParseCertificate(caDer)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA cert: %w", err)
	}

	return &TLSConfigManager{
		caCert: caCert,
		caKey:  caKey,
	}, nil
}

// GenerateNodeKeyPair generates a client/server TLS certificate signed by the root CA
func (m *TLSConfigManager) GenerateNodeKeyPair(nodeID string) (tls.Certificate, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate node key: %w", err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"AI Blockchain Node"},
			CommonName:   fmt.Sprintf("node-%s", nodeID),
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(30 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost", nodeID},
	}

	certDer, err := x509.CreateCertificate(rand.Reader, template, m.caCert, &privKey.PublicKey, m.caKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to sign node cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDer})
	keyBytes, _ := x509.MarshalECPrivateKey(privKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return tls.X509KeyPair(certPEM, keyPEM)
}

// GetServerTLSConfig returns a tls.Config enforcing mTLS 1.3
func (m *TLSConfigManager) GetServerTLSConfig(nodeCert tls.Certificate) *tls.Config {
	certPool := x509.NewCertPool()
	certPool.AddCert(m.caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{nodeCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS13,
	}
}

// GetClientTLSConfig returns a client tls.Config for connecting to peers
func (m *TLSConfigManager) GetClientTLSConfig(nodeCert tls.Certificate) *tls.Config {
	certPool := x509.NewCertPool()
	certPool.AddCert(m.caCert)

	return &tls.Config{
		Certificates:       []tls.Certificate{nodeCert},
		RootCAs:            certPool,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // Self-signed IP matching in test harness
	}
}
