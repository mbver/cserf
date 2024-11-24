package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ToTagMap(s string) map[string]string {
	m := map[string]string{}
	kvs := strings.Split(s, ",")
	for _, kv := range kvs {
		pair := strings.Split(kv, "=")
		if len(pair) < 2 {
			continue
		}
		m[pair[0]] = pair[1]
	}
	return m
}

func ToNodes(s string) []string {
	if len(s) == 0 {
		return nil
	}
	return strings.Split(s, ",")
}

func WaitForTerm(stop chan struct{}) {
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	select {
	case <-sigCh:
		return
	case <-stop:
		return

	}
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

func GenerateSelfSignedCert(certfile, keyfile string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
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
		return err
	}

	// Encode the private key and certificate as PEM
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certfile)
	if err != nil {
		return err
	}
	defer certOut.Close()

	keyOut, err := os.Create(keyfile)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}
	return nil
}
