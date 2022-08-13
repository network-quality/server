package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	_ "net/http/pprof"
)

var (
	configPort = flag.Int("config-port", 4043, "The port to listen on for generating config responses")
	publicPort = flag.Int("public-port", 4043, "The port to listen on for measurement accesses")
	listenAddr = flag.String("listen-addr", "localhost", "address to bind to")

	debug = flag.Bool("debug", false, "enable debug mode")

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

	var wg sync.WaitGroup
	wg.Add(1)

	listenAddr := fmt.Sprintf("%s:%d", *listenAddr, *configPort)
	go func(listenAddr string) {
		log.Printf("Network Quality URL: https://%s:%d%s/config", *configName, *configPort, *contextPath)
		if err := http.ListenAndServeTLS(listenAddr, *certFilename, *keyFilename, mux); err != nil {
			log.Fatal(err)
		}
		wg.Done()
	}(listenAddr)

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
