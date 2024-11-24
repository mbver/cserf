package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	mrand "math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	memberlist "github.com/mbver/mlist"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func CombineCleanup(cleanups ...func()) func() {
	return func() {
		for _, f := range cleanups {
			f()
		}
	}
}

// TODO: for testing, will remove
func testMemberlistConfig() *memberlist.Config {
	conf := memberlist.DefaultLANConfig()
	conf.PingTimeout = 20 * time.Millisecond
	conf.ProbeInterval = 60 * time.Millisecond
	conf.ProbeInterval = 5 * time.Millisecond
	conf.GossipInterval = 5 * time.Millisecond
	conf.PushPullInterval = 0
	conf.ReapInterval = 0
	return conf
}

// remove
func tmpPath() string {
	f := strconv.Itoa(mrand.Int())
	return filepath.Join(os.TempDir(), f)
}

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

func GenerateSelfSignedCert() (string, string, func(), error) {
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

func ShouldStopStreaming(err error) bool {
	st, ok := status.FromError(err)
	if ok {
		if st.Code() == codes.Unavailable {
			return true
		}
	}
	if err == io.EOF {
		return true
	}
	return false
}
