package rejectcontries

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateConfig(t *testing.T) {
	config := CreateConfig()
	
	if config.DBPath != "/mmdb/GeoLite2-Country.mmdb" {
		t.Errorf("Expected default DBPath to be '/mmdb/GeoLite2-Country.mmdb', got %s", config.DBPath)
	}
	
	if config.PreferXForwardedForHeader != true {
		t.Errorf("Expected default PreferXForwardedForHeader to be true, got %t", config.PreferXForwardedForHeader)
	}
	
	if len(config.MatchCountries) != 0 {
		t.Errorf("Expected default MatchCountries to be empty, got %v", config.MatchCountries)
	}
	
	if config.StaticResponse.StatusCode != http.StatusOK {
		t.Errorf("Expected default StatusCode to be %d, got %d", http.StatusOK, config.StaticResponse.StatusCode)
	}
}

func TestNew_EmptyMatchCountries(t *testing.T) {
	config := &RejectCountriesConfig{
		DBPath:                    "/non/existent/path.mmdb",
		PreferXForwardedForHeader: true,
		MatchCountries:            []string{},
		StaticResponse: StaticResponseConfig{
			StatusCode: 200,
			Headers:    http.Header{"Content-Type": []string{"text/plain"}},
			Body:       "Access denied",
		},
	}
	
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})
	
	_, err := New(context.Background(), next, config, "test")
	if err == nil {
		t.Error("Expected error for empty matchCountries, got nil")
	}
}

func TestNew_ValidConfig(t *testing.T) {
	config := &RejectCountriesConfig{
		DBPath:                    "/non/existent/path.mmdb", // Will fail to load but that's OK for this test
		PreferXForwardedForHeader: true,
		MatchCountries:            []string{"GB", "US"},
		StaticResponse: StaticResponseConfig{
			StatusCode: 403,
			Headers:    http.Header{"Content-Type": []string{"text/plain"}},
			Body:       "Access denied",
		},
	}
	
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})
	
	plugin, err := New(context.Background(), next, config, "test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	rejectCountries, ok := plugin.(*RejectCountries)
	if !ok {
		t.Error("Expected plugin to be *RejectCountries")
	}
	
	if !rejectCountries.matchCountries["GB"] {
		t.Error("Expected GB to be in matchCountries")
	}
	
	if !rejectCountries.matchCountries["US"] {
		t.Error("Expected US to be in matchCountries")
	}
}

func TestRejectCountries_ServeHTTP_NoGeoIP(t *testing.T) {
	config := &RejectCountriesConfig{
		DBPath:                    "/non/existent/path.mmdb",
		PreferXForwardedForHeader: true,
		MatchCountries:            []string{"GB"},
		StaticResponse: StaticResponseConfig{
			StatusCode: 403,
			Headers:    http.Header{"Content-Type": []string{"text/plain"}},
			Body:       "Access denied",
		},
	}
	
	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusOK)
	})
	
	plugin, err := New(context.Background(), next, config, "test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rw := httptest.NewRecorder()
	
	plugin.ServeHTTP(rw, req)
	
	if !nextCalled {
		t.Error("Expected next handler to be called when GeoIP is not available")
	}
}

func TestRejectCountries_GetClientIP(t *testing.T) {
	config := &RejectCountriesConfig{
		DBPath:                    "/non/existent/path.mmdb",
		PreferXForwardedForHeader: true,
		MatchCountries:            []string{"GB"},
		StaticResponse: StaticResponseConfig{
			StatusCode: 403,
			Body:       "Access denied",
		},
	}
	
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})
	
	plugin, err := New(context.Background(), next, config, "test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	rejectCountries := plugin.(*RejectCountries)
	
	// Test X-Forwarded-For header
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
	req.RemoteAddr = "127.0.0.1:12345"
	
	ip := rejectCountries.getClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("Expected IP from X-Forwarded-For to be '192.168.1.1', got '%s'", ip)
	}
	
	// Test RemoteAddr fallback
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	
	ip2 := rejectCountries.getClientIP(req2)
	if ip2 != "127.0.0.1" {
		t.Errorf("Expected IP from RemoteAddr to be '127.0.0.1', got '%s'", ip2)
	}
	
	// Test with PreferXForwardedForHeader disabled
	config.PreferXForwardedForHeader = false
	plugin2, _ := New(context.Background(), next, config, "test")
	rejectCountries2 := plugin2.(*RejectCountries)
	
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.Header.Set("X-Forwarded-For", "192.168.1.1")
	req3.RemoteAddr = "127.0.0.1:12345"
	
	ip3 := rejectCountries2.getClientIP(req3)
	if ip3 != "127.0.0.1" {
		t.Errorf("Expected IP to be '127.0.0.1' when X-Forwarded-For is disabled, got '%s'", ip3)
	}
}

func TestRejectCountries_ServeStaticResponse(t *testing.T) {
	config := &RejectCountriesConfig{
		DBPath:                    "/non/existent/path.mmdb",
		PreferXForwardedForHeader: true,
		MatchCountries:            []string{"GB"},
		StaticResponse: StaticResponseConfig{
			StatusCode: 403,
			Headers:    http.Header{"Content-Type": []string{"text/plain"}, "X-Custom": []string{"test"}},
			Body:       "Access denied from your country",
		},
	}
	
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})
	
	plugin, err := New(context.Background(), next, config, "test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	rejectCountries := plugin.(*RejectCountries)
	
	req := httptest.NewRequest("GET", "/", nil)
	rw := httptest.NewRecorder()
	
	rejectCountries.serveStaticResponse(rw, req)
	
	if rw.Code != 403 {
		t.Errorf("Expected status code 403, got %d", rw.Code)
	}
	
	if rw.Body.String() != "Access denied from your country" {
		t.Errorf("Expected body 'Access denied from your country', got '%s'", rw.Body.String())
	}
	
	if rw.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", rw.Header().Get("Content-Type"))
	}
	
	if rw.Header().Get("X-Custom") != "test" {
		t.Errorf("Expected X-Custom 'test', got '%s'", rw.Header().Get("X-Custom"))
	}
} 