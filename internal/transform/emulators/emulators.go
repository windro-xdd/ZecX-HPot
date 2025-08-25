package emulators

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"golang.org/x/crypto/ssh"
)

var serverStopCh = make(chan struct{})

// Start launches the high-interaction service emulators as concurrent goroutines.
func Start() error {
	log.Println("Starting service emulators...")

	go startSSHEmulator("0.0.0.0:2222")
	go startHTTPEmulator("0.0.0.0:8080")
	go startFTPEmulator("0.0.0.0:2121")

	fmt.Println("Service emulators started.")

	// Wait for a stop signal
	<-serverStopCh
	return nil
}

// Stop gracefully shuts down all running emulators.
func Stop() error {
	log.Println("Stopping all service emulators...")
	close(serverStopCh)
	return nil
}

// --- SSH Emulator ---
func startSSHEmulator(addr string) {
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			log.Printf("[SSH] Login attempt: user=%s, pass=%s from %s", c.User(), string(pass), c.RemoteAddr())
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	privateBytes, err := os.ReadFile("id_rsa_honeypot")
	if err != nil {
		// For simplicity, we generate a new key if one doesn't exist.
		// In a real scenario, you'd have a persistent, pre-generated key.
		private, err := ssh.ParsePrivateKey(generatePrivateKey())
		if err != nil {
			log.Fatalf("[SSH] Failed to parse private key: %v", err)
		}
		config.AddHostKey(private)
	} else {
		private, err := ssh.ParsePrivateKey(privateBytes)
		if err != nil {
			log.Fatalf("[SSH] Failed to parse private key file: %v", err)
		}
		config.AddHostKey(private)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[SSH] Failed to listen on %s: %v", addr, err)
	}
	log.Printf("[SSH] Listening on %s", addr)

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Printf("[SSH] Failed to accept incoming connection: %v", err)
			continue
		}
		go handleSSHConnection(nConn, config)
	}
}

func handleSSHConnection(nConn net.Conn, config *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		log.Printf("[SSH] Failed to handshake (%s)", err)
		return
	}
	log.Printf("[SSH] New SSH connection from %s (%s)", nConn.RemoteAddr(), "version_here")

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("[SSH] Could not accept channel: %v", err)
			continue
		}

		go func(in <-chan *ssh.Request) {
			for req := range in {
				// We are not implementing a full shell, just logging requests.
				log.Printf("[SSH] Request type: %s, Payload: %s", req.Type, string(req.Payload))
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}(requests)

		// A very basic "shell"
		channel.Write([]byte("Welcome to the honeypot.\r\n$ "))
		io.Copy(io.Discard, channel) // Read and discard all input
	}
}

func generatePrivateKey() []byte {
	// Generate a 2048-bit RSA key at runtime and return it in PEM format.
	// This avoids embedding a static private key in the repo while still
	// providing a valid key for the SSH emulator during testing.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("[SSH] Failed to generate private key: %v", err)
	}
	privDER := x509.MarshalPKCS1PrivateKey(priv)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privDER,
	}
	return pem.EncodeToMemory(pemBlock)
}

// --- HTTP Emulator ---
func startHTTPEmulator(addr string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[HTTP] Request from %s: %s %s", r.RemoteAddr, r.Method, r.URL.String())
		// Serve a fake "404 Not Found" page to most requests
		http.NotFound(w, r)
	})
	log.Printf("[HTTP] Listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Printf("[HTTP] Server error: %v", err)
	}
}

// --- FTP Emulator (Basic Listener) ---
func startFTPEmulator(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("[FTP] Failed to listen on %s: %v", addr, err)
	}
	log.Printf("[FTP] Listening on %s", addr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleFTPConnection(conn)
	}
}

func handleFTPConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("[FTP] Connection from %s", conn.RemoteAddr())
	conn.Write([]byte("220 ProFTPD 1.3.5a Server (Debian) [::ffff:127.0.0.1]\r\n"))
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		log.Printf("[FTP] Received from %s: %s", conn.RemoteAddr(), string(buf[:n]))
		// Simple canned responses
		if n > 0 {
			conn.Write([]byte("530 Please login with USER and PASS.\r\n"))
		}
	}
}
