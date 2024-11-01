package rpc

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
	"testing"
	"time"

	"github.com/mbver/cserf/rpc/client"
	"github.com/mbver/cserf/rpc/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
)

func generateSelfSignedCert() (tls.Certificate, *x509.Certificate, error) {
	// Generate a private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	// Create a certificate template
	certTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"My Organization"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour), // 1 year valid

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add Subject Alternative Names (SANs)
	certTemplate.DNSNames = []string{"localhost"}
	certTemplate.IPAddresses = []net.IP{[]byte{127, 0, 0, 1}} // for checking addresses resolved by dns

	// Create the certificate using the template and the private key (self-signed)
	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	// Encode the private key and certificate as PEM
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Create a TLS certificate for use with gRPC
	tlsCert, err := tls.X509KeyPair(certPEM, privPEM)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	// Parse the certificate to get the x509 certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	return tlsCert, cert, nil
}

func TestRPC_MismatchedCerts(t *testing.T) {
	addr := "localhost:50051"
	scert, _, err := generateSelfSignedCert()
	require.Nil(t, err)
	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{scert},
	})

	s, err := server.CreateServer(addr, creds)
	require.Nil(t, err)
	defer s.Stop()

	_, ccert, err := generateSelfSignedCert()
	require.Nil(t, err)
	certPool := x509.NewCertPool()
	certPool.AddCert(ccert)
	creds = credentials.NewTLS(&tls.Config{
		RootCAs: certPool,
	})

	c, err := client.CreateClient(addr, creds)
	require.Nil(t, err)
	defer c.Close()

	_, err = c.Hello("world")
	require.NotNil(t, err)
}

func TestRPC_Hello(t *testing.T) {
	addr := "localhost:50051"
	scert, ccert, err := generateSelfSignedCert()
	require.Nil(t, err)
	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{scert},
	})

	s, err := server.CreateServer(addr, creds)
	require.Nil(t, err)
	defer s.Stop()

	certPool := x509.NewCertPool()
	certPool.AddCert(ccert)
	creds = credentials.NewTLS(&tls.Config{
		RootCAs: certPool,
	})

	c, err := client.CreateClient(addr, creds)
	require.Nil(t, err)
	defer c.Close()

	res, err := c.Hello("world")
	require.Nil(t, err)
	require.Contains(t, res, "world")

	res, err = c.HelloStream("world")
	require.Nil(t, err)
	for i := 0; i < 3; i++ {
		require.Contains(t, res, fmt.Sprintf("world%d", i))
	}
}
