package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	"strconv"
	"sync"
	"time"

	// Do *not* remove this import. Per https://pkg.go.dev/net/http/pprof:
	// The package is typically only imported for the side effect of registering
	// its HTTP handlers. The handled paths all begin with /debug/pprof/.
	_ "net/http/pprof"
	// See -debug for how we use it.
)

var (
	configPort = flag.Int("config-port", 4043, "The port to listen on for generating config responses")
	publicPort = flag.Int("public-port", 4043, "The port to listen on for measurement accesses")
	listenAddr = flag.String("listen-addr", "localhost", "address to bind to")

	debug = flag.Bool("debug", false, "enable debug mode")

	announce = flag.Bool("announce", false, "announce this server using DNS-SD")

	certFilename = flag.String("cert-file", "", "cert to use")
	keyFilename  = flag.String("key-file", "", "key to use")

	configName   = flag.String("config-name", "networkquality.example.com", "domain to generate config for")
	publicName   = flag.String("public-name", "", "host to generate config for (same as -config-name if not specified)")
	contextPath  = flag.String("context-path", "", "context-path if behind a reverse-proxy")
	templateName = flag.String("template", "config.json.in", "template json config")
)

const (
	smallContentLength int64 = 1
	largeContentLength int64 = 4 * 1024 * 1024 * 1024
	chunkSize          int64 = 64 * 1024
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

func main() {
	flag.Parse()

	operatingCtx, operatingCtxCancel := context.WithCancel(context.Background())
	defer operatingCtxCancel()

	tmpl, err := template.ParseFiles(*templateName)
	if err != nil {
		log.Fatal(err)
	}

	if len(*certFilename) == 0 || len(*keyFilename) == 0 {
		log.Fatal("--cert-file and --key-file must be specified")
	}

	if len(*publicName) == 0 {
		*publicName = *configName
	}

	publicHostPort := fmt.Sprintf("%s:%d", *publicName, *publicPort)

	m := &Server{configName: *configName, publicHostPort: publicHostPort, contextPath: *contextPath, template: tmpl}

	mux := http.NewServeMux()
	mux.HandleFunc(m.contextPath+"/config", m.configHandler)
	mux.HandleFunc(m.contextPath+"/small", smallHandler)
	mux.HandleFunc(m.contextPath+"/large", largeHandler)
	mux.HandleFunc(m.contextPath+"/slurp", slurpHandler)

	if *debug {
		go func() {
			debugListenPort := 9090
			debugListenAddr := fmt.Sprintf("%s:%d", *listenAddr, debugListenPort)
			log.Println(http.ListenAndServe(debugListenAddr, nil))
		}()
	}

	if *announce {
		ips := make([]net.IP, 0)
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
		if announceResponder, announceHandle, announceErr := configureAnnouncer(ips, *configName, *configPort); announceErr == nil {
			defer announceResponder.Remove(announceHandle)
			go announceResponder.Respond(operatingCtx)
		} else {
			log.Printf("Warning: Could not announce the server instance: %v.\n", announceErr)
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)

	server := &http.Server{}
	server.Addr = fmt.Sprintf("%s:%d", *listenAddr, *configPort)
	server.Handler = mux

	go func(server *http.Server, configName string, configPort int, configContextPath string) {
		log.Printf("Network Quality URL: https://%s:%d%s/config", configName, configPort, configContextPath)
		if err := server.ListenAndServeTLS(*certFilename, *keyFilename); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
		wg.Done()
	}(server, *configName, *configPort, *contextPath)

	// The user can stop the server with SIGINT
	signalChannel := make(chan os.Signal, 1)   // make the channel buffered, per documentation.
	signal.Notify(signalChannel, os.Interrupt) // only Interrupt is guaranteed to exist on all platforms.

SignalLoop:
	for {
		select {
		case <-signalChannel:
			log.Printf("Shutting down the server ...\n")
			server.Shutdown(operatingCtx)
			break SignalLoop
		}
	}

	wg.Wait()
}

func (m *Server) configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Only generate the configuration from the json if we need to.
	if m.generatedConfig == nil {
		tv := tmplVars{
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
	setCors(w.Header())
	_, err := w.Write(m.generatedConfig.Bytes())
	if err != nil {
		log.Printf("could not write response: %s", err)
	}
}

func (m *Server) generateSmallDownloadURL() string {
	return fmt.Sprintf("https://%s%s/small", m.publicHostPort, m.contextPath)
}

func (m *Server) generateLargeDownloadURL() string {
	return fmt.Sprintf("https://%s%s/large", m.publicHostPort, m.contextPath)
}

func (m *Server) generateUploadURL() string {
	return fmt.Sprintf("https://%s%s/slurp", m.publicHostPort, m.contextPath)
}

type tmplVars struct {
	SmallDownloadURL string
	LargeDownloadURL string
	UploadURL        string
	ExternalHostname string
}

type Server struct {
	configName      string
	publicHostPort  string
	contextPath     string
	template        *template.Template
	generatedConfig *bytes.Buffer
}

func smallHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(smallContentLength, 10))
	w.Header().Set("Content-Type", "application/octet-stream")
	setCors(w.Header())

	if err := chunkedBodyWriter(w, smallContentLength); err != nil {
		log.Printf("Error writing content of length %d: %s", smallContentLength, err)
	}
}

func largeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(largeContentLength, 10))
	w.Header().Set("Content-Type", "application/octet-stream")
	setCors(w.Header())

	if err := chunkedBodyWriter(w, largeContentLength); err != nil {
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
func slurpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	setCors((w.Header()))
	setNoPublicCache(w.Header())

	t := time.Now()
	n, err := io.Copy(ioutil.Discard, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := struct {
		Duration time.Duration `json:"DurationMS"`
		Bytes    int64
		BPS      int64
	}{
		Duration: time.Since(t) / time.Millisecond,
		Bytes:    n,
	}

	if resp.Duration > 0 && resp.Bytes > 0 {
		resp.BPS = int64(float64(resp.Bytes) / (float64(resp.Duration) / 1000))
	}

	js, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	js = append(js, '\n')
	if _, err := w.Write(js); err != nil {
		log.Printf("ERROR: Could not write response: %s", err)
	}
}
