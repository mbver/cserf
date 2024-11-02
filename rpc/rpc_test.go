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
	"os"
	"testing"
	"time"

	"github.com/mbver/cserf/rpc/client"
	"github.com/mbver/cserf/rpc/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
)

func generateSelfSignedCert() (string, string, func(), error) {
	cleanup := func() {}
	// Generate a private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", cleanup, err
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
		return "", "", cleanup, err
	}

	// Encode the private key and certificate as PEM
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", cleanup, err
	}

	tempDir, err := os.MkdirTemp("", "certs")
	if err != nil {
		return "", "", cleanup, err
	}
	cleanup = func() { os.Remove(tempDir) }
	certFile := tempDir + "/server.crt"
	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", cleanup, err
	}
	defer certOut.Close()

	keyFile := tempDir + "/server.key"
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return "", "", cleanup, err
	}
	defer keyOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return "", "", cleanup, err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return "", "", cleanup, err
	}
	return certFile, keyFile, cleanup, nil
}

func TestRPC_MismatchedCerts(t *testing.T) {
	addr := "localhost:50051"
	certFile1, keyfile1, cleanup1, err := generateSelfSignedCert()
	defer cleanup1()
	require.Nil(t, err)

	serverCert, err := tls.LoadX509KeyPair(certFile1, keyfile1)
	require.Nil(t, err)

	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
	})

	s, err := server.CreateServer(addr, creds)
	require.Nil(t, err)
	defer s.Stop()

	certFile2, _, cleanup2, err := generateSelfSignedCert()
	defer cleanup2()
	require.Nil(t, err)

	caCert, err := os.ReadFile(certFile2)
	require.Nil(t, err)

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

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
	certFile, keyFile, cleanup, err := generateSelfSignedCert()
	defer cleanup()
	require.Nil(t, err)

	servertCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.Nil(t, err)
	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{servertCert},
	})

	s, err := server.CreateServer(addr, creds)
	require.Nil(t, err)
	defer s.Stop()

	caCert, err := os.ReadFile(certFile)
	require.Nil(t, err)
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)
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
