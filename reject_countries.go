// Package rejectcontries a demo plugin.
package rejectcontries

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/IncSW/geoip2"
)

// RejectCountriesConfig the plugin configuration.
type RejectCountriesConfig struct {
	DBPath                    string                     `json:"dbPath,omitempty"`
	PreferXForwardedForHeader bool                       `json:"preferXForwardedForHeader,omitempty"`
	MatchCountries            []string                   `json:"matchCountries,omitempty"`
	StaticResponse            StaticResponseConfig       `json:"staticResponse,omitempty"`
}

// StaticResponseConfig static response configuration.
type StaticResponseConfig struct {
	StatusCode int         `json:"statusCode,omitempty"`
	Headers    http.Header `json:"headers,omitempty"`
	Body       string      `json:"body,omitempty"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *RejectCountriesConfig {
	return &RejectCountriesConfig{
		DBPath:                    "/mmdb/GeoLite2-Country.mmdb",
		PreferXForwardedForHeader: true,
		MatchCountries:            []string{},
		StaticResponse: StaticResponseConfig{
			StatusCode: http.StatusOK,
			Headers:    http.Header{},
			Body:       "",
		},
	}
}

// RejectCountries plugin.
type RejectCountries struct {
	next                      http.Handler
	name                      string
	preferXForwardedForHeader bool
	matchCountries            map[string]bool
	staticResponse            StaticResponseConfig
	lookup                    RejectCountriesLookup
}

// RejectCountriesLookup lookup function type.
type RejectCountriesLookup func(ip net.IP) (string, error)

// New creates a new RejectCountries plugin.
func New(ctx context.Context, next http.Handler, config *RejectCountriesConfig, name string) (http.Handler, error) {
	if len(config.MatchCountries) == 0 {
		return nil, fmt.Errorf("matchCountries cannot be empty")
	}

	// Convert matchCountries slice to map for faster lookup
	matchCountriesMap := make(map[string]bool)
	for _, country := range config.MatchCountries {
		matchCountriesMap[strings.ToUpper(country)] = true
	}

	// Ensure static response headers are not nil
	if config.StaticResponse.Headers == nil {
		config.StaticResponse.Headers = http.Header{}
	}

	// Validate static response status code
	if config.StaticResponse.StatusCode < 100 || config.StaticResponse.StatusCode > 999 {
		config.StaticResponse.StatusCode = http.StatusOK
	}

	plugin := &RejectCountries{
		next:                      next,
		name:                      name,
		preferXForwardedForHeader: config.PreferXForwardedForHeader,
		matchCountries:            matchCountriesMap,
		staticResponse:            config.StaticResponse,
	}

	// Initialize GeoIP lookup
	if err := plugin.initGeoIPLookup(config.DBPath); err != nil {
		log.Printf("[RejectCountries] Failed to initialize GeoIP lookup: %v", err)
		// Continue without GeoIP lookup - plugin will allow all requests
	}

	return plugin, nil
}

// initGeoIPLookup initializes the GeoIP lookup function.
func (rc *RejectCountries) initGeoIPLookup(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("GeoIP database not found: %s", dbPath)
	}

	rdr, err := geoip2.NewCountryReaderFromFile(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open GeoIP database: %w", err)
	}

	rc.lookup = func(ip net.IP) (string, error) {
		rec, err := rdr.Lookup(ip)
		if err != nil {
			return "", err
		}
		return rec.Country.ISOCode, nil
	}

	log.Printf("[RejectCountries] GeoIP lookup initialized: db=%s, name=%s", dbPath, rc.name)
	return nil
}

// ServeHTTP processes the HTTP request.
func (rc *RejectCountries) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// If GeoIP lookup is not available, continue with request
	if rc.lookup == nil {
		rc.next.ServeHTTP(rw, req)
		return
	}

	// Get client IP
	ipStr := rc.getClientIP(req)
	if ipStr == "" {
		rc.next.ServeHTTP(rw, req)
		return
	}

	// Parse IP
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Printf("[RejectCountries] Invalid IP address: %s", ipStr)
		rc.next.ServeHTTP(rw, req)
		return
	}

	// Lookup country
	country, err := rc.lookup(ip)
	if err != nil {
		log.Printf("[RejectCountries] GeoIP lookup failed for IP %s: %v", ipStr, err)
		rc.next.ServeHTTP(rw, req)
		return
	}

	// Check if country should be rejected
	if rc.matchCountries[strings.ToUpper(country)] {
		log.Printf("[RejectCountries] Rejecting request from country %s (IP: %s)", country, ipStr)
		rc.serveStaticResponse(rw, req)
		return
	}

	// Continue with the request
	rc.next.ServeHTTP(rw, req)
}

// getClientIP extracts the client IP address from the request.
func (rc *RejectCountries) getClientIP(req *http.Request) string {
	if rc.preferXForwardedForHeader {
		// Check X-Forwarded-For header first
		forwardedFor := req.Header.Get("X-Forwarded-For")
		if forwardedFor != "" {
			ips := strings.Split(forwardedFor, ",")
			return strings.TrimSpace(ips[0])
		}
	}

	// If X-Forwarded-For is not present or retrieval is not enabled, fallback to RemoteAddr
	remoteAddr := req.RemoteAddr
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		remoteAddr = host
	}
	return remoteAddr
}

// serveStaticResponse sends the configured static response.
func (rc *RejectCountries) serveStaticResponse(rw http.ResponseWriter, req *http.Request) {
	// Set headers first before sending the response
	for key, values := range rc.staticResponse.Headers {
		for _, value := range values {
			rw.Header().Add(key, value)
		}
	}

	rw.WriteHeader(rc.staticResponse.StatusCode)

	if rc.staticResponse.Body != "" {
		_, _ = rw.Write([]byte(rc.staticResponse.Body))
	}
} 