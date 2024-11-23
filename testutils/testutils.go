package testutils

// import (
// 	"crypto/ecdsa"
// 	"crypto/elliptic"
// 	"crypto/rand"
// 	"crypto/tls"
// 	"crypto/x509"
// 	"crypto/x509/pkix"
// 	"encoding/pem"
// 	"fmt"
// 	"log"
// 	"math/big"
// 	mrand "math/rand"
// 	"net"
// 	"os"
// 	"path/filepath"
// 	"strconv"
// 	"sync/atomic"
// 	"time"

// 	serf "github.com/mbver/cserf"
// 	"github.com/mbver/cserf/rpc/client"
// 	"github.com/mbver/cserf/rpc/server"
// 	memberlist "github.com/mbver/mlist"
// 	"github.com/mbver/mlist/testaddr"
// 	"google.golang.org/grpc"
// 	"google.golang.org/grpc/credentials"
// )

// func createTestEventScript() (string, func(), error) {
// 	cleanup := func() {}
// 	tmp, err := os.CreateTemp("", "*script.sh")
// 	if err != nil {
// 		return "", cleanup, err
// 	}
// 	defer tmp.Close()
// 	cleanup = func() {
// 		os.Remove(tmp.Name())
// 	}
// 	if _, err := tmp.Write([]byte(`echo "Hello"`)); err != nil {
// 		return "", cleanup, err
// 	}
// 	if err := os.Chmod(tmp.Name(), 0755); err != nil {
// 		fmt.Println("Error making temp file executable:", err)
// 		return "", cleanup, err
// 	}
// 	return tmp.Name(), cleanup, nil
// }

// func testMemberlistConfig() *memberlist.Config {
// 	conf := memberlist.DefaultLANConfig()
// 	conf.PingTimeout = 20 * time.Millisecond
// 	conf.ProbeInterval = 60 * time.Millisecond
// 	conf.ProbeInterval = 5 * time.Millisecond
// 	conf.GossipInterval = 5 * time.Millisecond
// 	conf.PushPullInterval = 0
// 	conf.ReapInterval = 0
// 	return conf
// }

// func CombineCleanup(cleanups ...func()) func() {
// 	return func() {
// 		for _, f := range cleanups {
// 			f()
// 		}
// 	}
// }

// type TestNodeOpts struct {
// 	Tags          map[string]string
// 	IP            net.IP
// 	Port          int
// 	Snap          string
// 	Script        string
// 	Ping          serf.PingDelegate
// 	Keyring       *memberlist.Keyring
// 	TombStone     time.Duration
// 	Coalesce      time.Duration
// 	FailedTimeout time.Duration
// 	Reconnect     time.Duration
// 	Logger        *log.Logger
// }

// func tmpPath() string {
// 	f := strconv.Itoa(mrand.Int())
// 	return filepath.Join(os.TempDir(), f)
// }

// func TestNode(opts *TestNodeOpts) (*serf.Serf, func(), error) {
// 	if opts == nil {
// 		opts = &TestNodeOpts{}
// 	}
// 	b := &serf.SerfBuilder{}
// 	cleanup := func() {}

// 	keyring := opts.Keyring
// 	var err error
// 	if keyring == nil {
// 		key := []byte{79, 216, 231, 114, 9, 125, 153, 178, 238, 179, 230, 218, 77, 54, 187, 171, 185, 207, 73, 74, 215, 193, 176, 226, 217, 216, 91, 182, 168, 171, 223, 187}
// 		keyring, err = memberlist.NewKeyring(nil, key)
// 		if err != nil {
// 			return nil, cleanup, err
// 		}
// 	}
// 	b.WithKeyring(keyring)

// 	ip := opts.IP
// 	if ip == nil {
// 		ip, cleanup = testaddr.BindAddrs.NextAvailAddr()
// 	}
// 	mconf := testMemberlistConfig()
// 	mconf.BindAddr = ip.String()
// 	port := opts.Port
// 	if port != 0 {
// 		mconf.BindPort = port
// 	}
// 	mconf.Label = "label"
// 	b.WithMemberlistConfig(mconf)

// 	logger := opts.Logger
// 	if logger == nil {
// 		prefix := fmt.Sprintf("serf-%s: ", mconf.BindAddr)
// 		logger = log.New(os.Stderr, prefix, log.LstdFlags)
// 	}
// 	b.WithLogger(logger)

// 	snapPath := opts.Snap
// 	if snapPath == "" {
// 		snapPath = tmpPath()
// 	}
// 	script := opts.Script
// 	if script == "" {
// 		var cleanup1, cleanup2 func()
// 		script, cleanup1, err = createTestEventScript()
// 		cleanup2 = CombineCleanup(cleanup, cleanup1)
// 		cleanup = cleanup2
// 		if err != nil {
// 			return nil, cleanup, err
// 		}
// 	}
// 	conf := &serf.Config{
// 		EventScript:            script,
// 		LBufferSize:            1024,
// 		QueryTimeoutMult:       16,
// 		QueryResponseSizeLimit: 1024,
// 		QuerySizeLimit:         1024,
// 		ActionSizeLimit:        512,
// 		SnapshotPath:           snapPath,
// 		SnapshotMinCompactSize: 128 * 1024,
// 		SnapshotDrainTimeout:   500 * time.Millisecond,
// 		CoalesceInterval:       5 * time.Millisecond,
// 		ReapInterval:           10 * time.Millisecond,
// 		// ReconnectInterval:      1 * time.Millisecond,
// 		MaxQueueDepth:    1024,
// 		ReconnectTimeout: 5 * time.Millisecond,
// 		TombstoneTimeout: 5 * time.Millisecond,
// 	} // fill in later
// 	if opts.TombStone > 0 {
// 		conf.TombstoneTimeout = opts.TombStone
// 	}
// 	if opts.Coalesce > 0 {
// 		conf.CoalesceInterval = opts.Coalesce
// 	}
// 	if opts.FailedTimeout > 0 {
// 		conf.ReconnectTimeout = opts.FailedTimeout
// 	}
// 	if opts.Reconnect > 0 {
// 		conf.ReconnectInterval = opts.Reconnect
// 	}
// 	cleanup1 := CombineCleanup(cleanup, func() {
// 		data, _ := os.ReadFile(conf.SnapshotPath)
// 		logger.Printf("### snapshot %s:", string(data))
// 		os.Remove(conf.SnapshotPath)
// 	})

// 	b.WithConfig(conf)

// 	b.WithTags(opts.Tags)

// 	b.WithPingDelegate(opts.Ping)

// 	s, err := b.Build()
// 	if err != nil {
// 		return nil, cleanup1, err
// 	}
// 	cleanup2 := CombineCleanup(s.Shutdown, cleanup1)
// 	// if opts.eventCh != nil {
// 	// 	time.Sleep(50 * time.Millisecond) // wait for initial events flushed out
// 	// 	stream := serf.StreamEventHandler{
// 	// 		eventCh: opts.eventCh,
// 	// 	}
// 	// 	s.eventHandlers.stream.register(&stream)
// 	// }
// 	return s, cleanup2, nil
// }

// func TwoNodes(opts1, opts2 *TestNodeOpts) (*serf.Serf, *serf.Serf, func(), error) {
// 	s1, cleanup1, err := TestNode(opts1)
// 	if err != nil {
// 		return nil, nil, cleanup1, err
// 	}
// 	s2, cleanup2, err := TestNode(opts2)
// 	cleanup := CombineCleanup(cleanup1, cleanup2)
// 	if err != nil {
// 		return nil, nil, cleanup, err
// 	}
// 	return s1, s2, cleanup, err
// }

// func TwoNodesJoined(opts1, opts2 *TestNodeOpts) (*serf.Serf, *serf.Serf, func(), error) {
// 	s1, s2, cleanup, err := TwoNodes(opts1, opts2)
// 	if err != nil {
// 		return nil, nil, cleanup, err
// 	}
// 	addr, err := s2.AdvertiseAddress()
// 	if err != nil {
// 		return nil, nil, cleanup, err
// 	}
// 	n, err := s1.Join([]string{addr}, false)
// 	if err != nil {
// 		return nil, nil, cleanup, err
// 	}
// 	if n != 1 {
// 		return nil, nil, cleanup, fmt.Errorf("join failed")
// 	}
// 	return s1, s2, cleanup, err
// }

// func Retry(times int, fn func() (bool, string)) (success bool, msg string) {
// 	for i := 0; i < times; i++ {
// 		success, msg = fn()
// 		if success {
// 			return
// 		}
// 	}
// 	return
// }

// var rpcPort uint32 = 50050

// func NextRpcPort() uint32 {
// 	return atomic.AddUint32(&rpcPort, 1)
// }

// func GenerateSelfSignedCert() (string, string, func(), error) {
// 	cleanup := func() {}
// 	// Generate a private key
// 	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
// 	if err != nil {
// 		return "", "", cleanup, err
// 	}

// 	// Create a certificate template
// 	certTemplate := x509.Certificate{
// 		SerialNumber: big.NewInt(1),
// 		Subject: pkix.Name{
// 			Organization: []string{"My Organization"},
// 		},
// 		NotBefore: time.Now(),
// 		NotAfter:  time.Now().Add(365 * 24 * time.Hour), // 1 year valid

// 		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
// 		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
// 		BasicConstraintsValid: true,
// 	}

// 	// Add Subject Alternative Names (SANs)
// 	certTemplate.DNSNames = []string{"localhost"}
// 	certTemplate.IPAddresses = []net.IP{[]byte{127, 0, 0, 1}} // for checking addresses resolved by dns

// 	// Create the certificate using the template and the private key (self-signed)
// 	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &priv.PublicKey, priv)
// 	if err != nil {
// 		return "", "", cleanup, err
// 	}

// 	// Encode the private key and certificate as PEM
// 	privBytes, err := x509.MarshalECPrivateKey(priv)
// 	if err != nil {
// 		return "", "", cleanup, err
// 	}

// 	tempDir, err := os.MkdirTemp("", "certs")
// 	if err != nil {
// 		return "", "", cleanup, err
// 	}
// 	cleanup = func() { os.Remove(tempDir) }
// 	certFile := tempDir + "/server.crt"
// 	certOut, err := os.Create(certFile)
// 	if err != nil {
// 		return "", "", cleanup, err
// 	}
// 	defer certOut.Close()

// 	keyFile := tempDir + "/server.key"
// 	keyOut, err := os.Create(keyFile)
// 	if err != nil {
// 		return "", "", cleanup, err
// 	}
// 	defer keyOut.Close()

// 	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
// 		return "", "", cleanup, err
// 	}

// 	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
// 		return "", "", cleanup, err
// 	}
// 	return certFile, keyFile, cleanup, nil
// }

// // TODO: consider put it back in rpc
// func PrepareRPCClient(addr string, cert string) (*client.Client, error) {
// 	caCert, err := os.ReadFile(cert)
// 	if err != nil {
// 		return nil, err
// 	}
// 	certPool := x509.NewCertPool()
// 	certPool.AppendCertsFromPEM(caCert)
// 	creds := credentials.NewTLS(&tls.Config{
// 		RootCAs: certPool,
// 	})
// 	c, err := client.CreateClient(addr, creds)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return c, nil
// }

// func CreateTestClient(addr string, certPath string) (*client.Client, error) {
// 	return PrepareRPCClient(addr, certPath)
// }

// func testServerConf() (*server.ServerConfig, func(), error) {
// 	ip, cleanup := testaddr.BindAddrs.NextAvailAddr()
// 	rPort := NextRpcPort()
// 	certPath, keyPath, cleanup1, err := GenerateSelfSignedCert()
// 	cleanup2 := CombineCleanup(cleanup1, cleanup)
// 	if err != nil {
// 		return nil, cleanup2, err
// 	}
// 	GenerateSelfSignedCert()
// 	return &server.ServerConfig{
// 		RpcAddress: "127.0.0.1",
// 		RpcPort:    int(rPort),
// 		BindAddr:   ip.String(),
// 		KeyPath:    certPath,
// 		CertPath:   keyPath,
// 	}, cleanup2, nil
// }

// func TestServer() (*grpc.Server, func(), error) {
// 	conf, cleanup, err := testServerConf()
// 	if err != nil {
// 		return nil, cleanup, err
// 	}
// 	return server.CreateServer(conf)
// }
