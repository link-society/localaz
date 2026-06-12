package armserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"localaz.dev/internal/stores/armstore"
)

// newTestServer returns an httptest server backed by a fresh ARM store.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(New(armstore.New(armstore.Config{})))
	t.Cleanup(ts.Close)
	return ts
}

const providerItemPath = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ServiceBus/namespaces/ns1"

// TestProviderItemDeleteStatus verifies DELETE returns 200 when the resource
// existed and 204 when it did not (mirroring the resource-group handler).
func TestProviderItemDeleteStatus(t *testing.T) {
	ts := newTestServer(t)
	client := ts.Client()

	// Create the resource first.
	putReq, _ := http.NewRequest(http.MethodPut, ts.URL+providerItemPath, strings.NewReader(`{}`))
	putResp, err := client.Do(putReq)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}

	// DELETE of an existing resource ⇒ 200.
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+providerItemPath, nil)
	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE existing: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE existing status = %d, want %d", delResp.StatusCode, http.StatusOK)
	}

	// DELETE of an absent resource ⇒ 204.
	del2Req, _ := http.NewRequest(http.MethodDelete, ts.URL+providerItemPath, nil)
	del2Resp, err := client.Do(del2Req)
	if err != nil {
		t.Fatalf("DELETE absent: %v", err)
	}
	del2Resp.Body.Close()
	if del2Resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE absent status = %d, want %d", del2Resp.StatusCode, http.StatusNoContent)
	}
}

// TestProviderItemPutBodyTooLarge verifies a PUT body larger than the
// MaxBytesReader cap is rejected with a 4xx rather than decoded and stored.
func TestProviderItemPutBodyTooLarge(t *testing.T) {
	ts := newTestServer(t)
	client := ts.Client()

	// A JSON object well past the 1 MiB cap.
	huge := `{"properties":{"blob":"` + strings.Repeat("a", 2<<20) + `"}}`
	putReq, _ := http.NewRequest(http.MethodPut, ts.URL+providerItemPath, strings.NewReader(huge))
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := client.Do(putReq)
	if err != nil {
		t.Fatalf("PUT oversized: %v", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode < 400 || putResp.StatusCode >= 500 {
		t.Fatalf("PUT oversized status = %d, want 4xx", putResp.StatusCode)
	}

	// The oversized body must not have been stored.
	getReq, _ := http.NewRequest(http.MethodGet, ts.URL+providerItemPath, nil)
	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("GET after oversized PUT: %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET after oversized PUT status = %d, want %d (resource should not exist)", getResp.StatusCode, http.StatusNotFound)
	}
}
