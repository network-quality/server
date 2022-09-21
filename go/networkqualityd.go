package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"strconv"
	"sync"

	// Do *not* remove this import. Per https://pkg.go.dev/net/http/pprof:
	// The package is typically only imported for the side effect of registering
	// its HTTP handlers. The handled paths all begin with /debug/pprof/.
	// See -debug for how we use it.
	_ "net/http/pprof"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	insecurePublicPort = flag.Int("insecure-public-port", 0, "The port to listen on for HTTP measurement accesses")
	publicPort         = flag.Int("public-port", 4043, "The port to listen on for HTTPS/H2C/HTTP3 measurement accesses")

	listenAddr = flag.String("listen-addr", "localhost", "address to bind to")

	announce    = flag.Bool("announce", false, "announce this server using DNS-SD")
	debug       = flag.Bool("debug", false, "enable debug mode")
	enableCORS  = flag.Bool("enable-cors", false, "enable CORS headers")
	enableH2C   = flag.Bool("enable-h2c", false, "enable h2c (non-TLS http/2 prior knowledge) mode")
	enableHTTP2 = flag.Bool("enable-http2", true, "enable HTTP/2")
	enableHTTP3 = flag.Bool("enable-http3", false, "enable HTTP/3")

	socketSendBuffer = flag.Uint("socket-send-buffer-size", 0, "The size of the socket send buffer via TCP_NOTSENT_LOWAT. Zero/unset means to leave unset")

	tosString    = flag.String("tos", "0", "set TOS for listening socket")
	certFilename = flag.String("cert-file", "", "cert to use")
	keyFilename  = flag.String("key-file", "", "key to use")

	configName   = flag.String("config-name", "networkquality.example.com", "domain to generate config for")
	publicName   = flag.String("public-name", "", "host to generate config for (same as -config-name if not specified)")
	contextPath  = flag.String("context-path", "", "context-path if behind a reverse-proxy")
	templateName = flag.String("template", "config.json.in", "template json config")
)

const (
	smallContentLength int64 = 1
	largeContentLength int64 = 32 * 1024 * 1024 * 1024
	chunkSize          int64 = 64 * 1024

	defaultInsecurePublicPort = 4080
)

var (
	buffed []byte
)

func init() {
	buffed = make([]byte, chunkSize)
	for i := range buffed {
		buffed[i] = 'x'
	}
}

// setCors makes it possible for wasm clients to connect to the server
// from a webclient that is not hosted on the same domain.
func setCors(h http.Header) {
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Headers", "*")
}

type handlers struct {
	enableCORS bool
}

// BulkHandlers returns path, handler tuples with the provided prefix.
func BulkHandlers(prefix string, enableCORS bool) map[string]http.HandlerFunc {
	h := &handlers{enableCORS: enableCORS}
	return map[string]http.HandlerFunc{
		prefix + "/small": h.smallHandler,
		prefix + "/large": h.largeHandler,
		prefix + "/slurp": h.slurpHandler,
	}
}

func main() {
	flag.Parse()

	tosTemp, err := strconv.ParseUint(*tosString, 10, 8)
	if err != nil {
		log.Fatal(err)
	}
	tos := uint8(tosTemp)

	operatingCtx, operatingCtxCancel := context.WithCancel(context.Background())
	defer operatingCtxCancel()

	tmpl, err := template.ParseFiles(*templateName)
	if err != nil {
		log.Fatal(err)
	}

	certSpecified := false
	if len(*certFilename) > 0 && len(*keyFilename) > 0 {
		certSpecified = true
	}

	var cfg *tls.Config
	if certSpecified {
		cfg = &tls.Config{}

		cfg.Certificates = make([]tls.Certificate, 1)
		cfg.Certificates[0], err = tls.LoadX509KeyPair(*certFilename, *keyFilename)
		if err != nil {
			log.Fatal(err)
		}

		if *enableHTTP2 {
			cfg.NextProtos = []string{"h2"}
		}

	}

	if len(*publicName) == 0 {
		*publicName = *configName
	}

	portScheme := make(map[int]string)
	if *enableH2C || !certSpecified {
		*insecurePublicPort = defaultInsecurePublicPort
		portScheme[*insecurePublicPort] = "http"
	} else {
		portScheme[*publicPort] = "https"
		if *insecurePublicPort > 0 {
			portScheme[*insecurePublicPort] = "http"
		}
	}

	if *debug {
		go func() {
			debugListenPort := 9090
			debugListenAddr := fmt.Sprintf("%s:%d", *listenAddr, debugListenPort)
			log.Println(http.ListenAndServe(debugListenAddr, nil))
		}()
	}

	var announceShutdowners []func()
	var servers []*http.Server

	var wg sync.WaitGroup
	wg.Add(len(portScheme))

	ips := make([]net.IP, 0)
	if *announce {
		// The user may give us a hostname (rather than an address to listen on). In order to
		// handle this situation, we will use DNS to convert it to an IP. As a result, we may
		// get back more than one address -- handle that!
		if addresses, lookupErr := net.LookupHost(*listenAddr); lookupErr == nil {
			for _, addr := range addresses {
				if parsedAddr := net.ParseIP(addr); parsedAddr != nil {
					ips = append(ips, parsedAddr)
				}
			}
		}
	}

	for port, scheme := range portScheme {
		var hostPort string
		if port == 80 || port == 443 {
			hostPort = *publicName
		} else {
			hostPort = fmt.Sprintf("%s:%d", *publicName, port)
		}

		m := &Server{
			publicHostPort: hostPort,
			template:       tmpl,
			enableCORS:     *enableCORS,
			contextPath:    *contextPath,
			scheme:         scheme,
		}

		mux := http.NewServeMux()
		mux.HandleFunc(m.contextPath+"/config", m.configHandler)
		for pattern, handler := range BulkHandlers(m.contextPath, *enableCORS) {
			mux.HandleFunc(pattern, handler)
		}

		var nl net.Listener
		var err error

		listenConfig := net.ListenConfig{
			Control: func(network, address string, conn syscall.RawConn) error {
				if *socketSendBuffer > 0 {
					log.Printf("setting TCP_NOTSENT_LOWAT to %d", *socketSendBuffer)
					if err := setTCPNotSentLowat(conn, int(*socketSendBuffer)); err != nil {
						return err
					}
				}

				if tos > 0 {
					log.Printf("Setting IP_TOS to %d", tos)
					if err := setIPTos(network, conn, int(tos)); err != nil {
						return err
					}
				}
				return nil
			},
		}

		nl, err = listenConfig.Listen(operatingCtx, "tcp", fmt.Sprintf("%s:%d", *listenAddr, port))
		if err != nil {
			log.Fatal(err)
		}

		if scheme == "https" {
			nl = tls.NewListener(nl, cfg)
		}

		mynl := nl

		log.Printf("Network Quality URL: %s://%s:%d%s/config", scheme, *configName, port, *contextPath)

		go func(scheme string, nl net.Listener, port int) {
			if *enableH2C {
				server := &http.Server{
					Handler: h2c.NewHandler(mux, &http2.Server{}),
				}
				servers = append(servers, server)
				if err := server.Serve(nl); err != nil {
					log.Fatal(err)
				}
				wg.Done()
			} else {
				if scheme == "https" {
					if *enableHTTP3 {
						log.Printf("Enabling H3 on %q", fmt.Sprintf("%s:%d", *listenAddr, port))
						server := http3.Server{
							Handler:    mux,
							Addr:       fmt.Sprintf("%s:%d", *listenAddr, port),
							QuicConfig: &quic.Config{},
						}
						// No Shutdown(...) available for http3.Server

						go func() {
							if err := server.ListenAndServeTLS(*certFilename, *keyFilename); !errors.Is(err, http.ErrServerClosed) {
								log.Fatal(err)
							}
							wg.Done()
						}()
					}
					server := &http.Server{
						Handler: mux,
					}

					if *enableHTTP2 {
						log.Printf("Enabling H2 on %q", fmt.Sprintf("%s:%d", *listenAddr, port))
						if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
							log.Fatal(err)
						}
					}
					servers = append(servers, server)

					if err := server.Serve(nl); !errors.Is(err, http.ErrServerClosed) {
						log.Fatalf("FATAL: %q", err)
					}
				} else {
					server := &http.Server{
						Handler: mux,
					}
					servers = append(servers, server)
					if err := server.Serve(nl); !errors.Is(err, http.ErrServerClosed) {
						log.Fatalf("FATAL: %q", err)
					}
				}
			}
			wg.Done()
		}(scheme, mynl, port)

		if *announce {
			announceResponder, announceHandle, err := configureAnnouncer(ips, *configName, port)
			if err != nil {
				log.Fatalf("Could not announce the server instance: %v", err)
			}

			go announceResponder.Respond(operatingCtx)
			announceShutdowners = append(announceShutdowners, func() { announceResponder.Remove(announceHandle) })
		}

	}

	// The user can stop the server with SIGINT
	signalChannel := make(chan os.Signal, 1)   // make the channel buffered, per documentation.
	signal.Notify(signalChannel, os.Interrupt) // only Interrupt is guaranteed to exist on all platforms.

	<-signalChannel

	for _, server := range servers {
		if err := server.Shutdown(operatingCtx); err != nil {
			log.Printf("error shuting down: %s", err)
		}
	}

	wg.Wait()

	if *announce {
		log.Printf("Shutting down dnssd announcer")
		shutdownDone := make(chan interface{})
		go func() {
			for _, shutdowner := range announceShutdowners {
				shutdowner()
			}
			shutdownDone <- nil
		}()

		// Either wait for Remove to complete or another SIGINT
		select {
		case <-signalChannel:
		case <-shutdownDone:
		}
	}
}

func (m *Server) configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Only generate the configuration from the json if we need to.
	if m.generatedConfig == nil {
		tv := struct {
			SmallDownloadURL string
			LargeDownloadURL string
			UploadURL        string
		}{
			SmallDownloadURL: m.generateSmallDownloadURL(),
			LargeDownloadURL: m.generateLargeDownloadURL(),
			UploadURL:        m.generateUploadURL(),
		}

		var b bytes.Buffer
		if err := m.template.Execute(&b, tv); err != nil {
			log.Printf("Error rendering config: %s", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		m.generatedConfig = &b
	}

	w.Header().Set("Content-Type", "application/json")

	if m.enableCORS {
		setCors(w.Header())
	}

	_, err := w.Write(m.generatedConfig.Bytes())
	if err != nil {
		log.Printf("could not write response: %s", err)
	}
}

func (m *Server) generateSmallDownloadURL() string {
	return fmt.Sprintf("%s://%s%s/small", m.scheme, m.publicHostPort, m.contextPath)
}

func (m *Server) generateLargeDownloadURL() string {
	return fmt.Sprintf("%s://%s%s/large", m.scheme, m.publicHostPort, m.contextPath)
}

func (m *Server) generateUploadURL() string {
	return fmt.Sprintf("%s://%s%s/slurp", m.scheme, m.publicHostPort, m.contextPath)
}

// A Server defines parameters for running a network quality server.
type Server struct {
	publicHostPort  string
	contextPath     string
	scheme          string
	template        *template.Template
	generatedConfig *bytes.Buffer
	enableCORS      bool
}

func (h *handlers) smallHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(smallContentLength, 10))
	w.Header().Set("Content-Type", "application/octet-stream")

	if h.enableCORS {
		setCors(w.Header())
	}

	if err := chunkedBodyWriter(w, smallContentLength); !ignorableError(err) {
		log.Printf("Error writing content of length %d: %s", smallContentLength, err)
	}
}

func (h *handlers) largeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(largeContentLength, 10))
	w.Header().Set("Content-Type", "application/octet-stream")

	if h.enableCORS {
		setCors(w.Header())
	}

	if err := chunkedBodyWriter(w, largeContentLength); !ignorableError(err) {
		log.Printf("Error writing content of length %d: %s", largeContentLength, err)
	}
}

func chunkedBodyWriter(w http.ResponseWriter, contentLength int64) error {
	w.WriteHeader(http.StatusOK)

	n := contentLength
	for n > 0 {
		if n >= chunkSize {
			n -= chunkSize
			if _, err := w.Write(buffed); err != nil {
				return err
			}
			continue
		}

		if _, err := w.Write(buffed[:n]); err != nil {
			return err
		}
		break
	}

	return nil
}

// setNoPublicCache tells the proxy to cache the content and the user
// that it can't be cached. It requires the proxy cache to be configured
// to use the Proxy-Cache-Control header
func setNoPublicCache(h http.Header) {
	h.Set("Proxy-Cache-Control", "max-age=604800, public")
	h.Set("Cache-Control", "no-store, must-revalidate, private, max-age=0")
}

// slurpHandler reads the post request and returns JSON with bytes
// read and how long it took
func (h *handlers) slurpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	setNoPublicCache(w.Header())

	if h.enableCORS {
		setCors((w.Header()))
	}

	_, err := io.Copy(ioutil.Discard, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(200)
}

// ignorableError returns true if error does not effect results of clients accessing server
func ignorableError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}

	switch err.Error() {
	case "client disconnected": // from http.http2errClientDisconnected
		return true
	}
	return false
}
