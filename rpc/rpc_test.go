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

	serf "github.com/mbver/cserf"
	"github.com/mbver/cserf/rpc/client"
	"github.com/mbver/cserf/rpc/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var testEventScript string

func createTestEventScript() (string, func(), error) {
	cleanup := func() {}
	tmp, err := os.CreateTemp("", "*script.sh")
	if err != nil {
		return "", cleanup, err
	}
	defer tmp.Close()
	cleanup = func() {
		os.Remove(tmp.Name())
	}
	if _, err := tmp.Write([]byte(`echo "Hello"`)); err != nil {
		return "", cleanup, err
	}
	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		fmt.Println("Error making temp file executable:", err)
		return "", cleanup, err
	}
	return tmp.Name(), cleanup, nil
}

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

var cert1, key1, cert2, key2 string

func TestMain(m *testing.M) {
	var err error
	var cleanup1, cleanup2 func()

	cert1, key1, cleanup1, err = generateSelfSignedCert()
	defer cleanup1()
	if err != nil {
		panic(err)
	}

	cert2, key2, cleanup2, err = generateSelfSignedCert()
	defer cleanup2()
	if err != nil {
		panic(err)
	}

	tmp, cleanup3, err := createTestEventScript()
	defer cleanup3()
	if err != nil {
		panic(err)
	}
	testEventScript = tmp

	m.Run()
}

func prepareRPCServer(addr string, cert, key string, serf *serf.Serf) (*grpc.Server, error) {
	serverCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
	})
	s, err := server.CreateServer(addr, creds, serf)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func prepareRPCClient(addr string, cert string) (*client.Client, error) {
	caCert, err := os.ReadFile(cert)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)
	creds := credentials.NewTLS(&tls.Config{
		RootCAs: certPool,
	})
	c, err := client.CreateClient(addr, creds)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func TestRPC_MismatchedCerts(t *testing.T) {
	addr := fmt.Sprintf("localhost:%d", nextRpcPort())

	server, err := prepareRPCServer(addr, cert1, key1, nil)
	require.Nil(t, err)
	defer server.Stop()

	client, err := prepareRPCClient(addr, cert2)
	require.Nil(t, err)
	defer client.Close()

	_, err = client.Hello("world")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "connection refused")
}

func TestRPC_Hello(t *testing.T) {
	addr := fmt.Sprintf("localhost:%d", nextRpcPort())

	server, err := prepareRPCServer(addr, cert1, key1, nil)
	require.Nil(t, err)
	defer server.Stop()

	client, err := prepareRPCClient(addr, cert1)
	require.Nil(t, err)
	defer client.Close()

	res, err := client.Hello("world")
	require.Nil(t, err)
	require.Contains(t, res, "world")

	res, err = client.HelloStream("world")
	require.Nil(t, err)
	for i := 0; i < 3; i++ {
		require.Contains(t, res, fmt.Sprintf("world%d", i))
	}
}

func TestRPC_Query(t *testing.T) {
	s1, s2, s3, cleanup, err := threeNodes()
	defer cleanup()
	require.Nil(t, err)

	addr2, err := s2.AdvertiseAddress()
	require.Nil(t, err)

	addr3, err := s3.AdvertiseAddress()
	require.Nil(t, err)

	n, err := s1.Join([]string{addr2, addr3}, false)
	require.Nil(t, err)
	require.Equal(t, 2, n)

	addr := fmt.Sprintf("localhost:%d", nextRpcPort())

	server, err := prepareRPCServer(addr, cert1, key1, s1)
	require.Nil(t, err)
	defer server.Stop()

	client, err := prepareRPCClient(addr, cert1)
	require.Nil(t, err)
	defer client.Close()

	res, err := client.Query(nil)
	require.Nil(t, err)

	require.Contains(t, res, s1.ID())
	require.Contains(t, res, s2.ID())
	require.Contains(t, res, s3.ID())
}
