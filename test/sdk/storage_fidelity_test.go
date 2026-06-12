package sdk

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"localaz.dev/internal/servers/queueserver"
	"localaz.dev/internal/servers/tableserver"
	"localaz.dev/internal/stores/queuestore"
	"localaz.dev/internal/stores/tablestore"
)

// newQueueServer spins up an in-process Queue emulator and returns its base URL
// (including the development account segment) for raw HTTP requests.
func newQueueServer(t *testing.T) string {
	t.Helper()
	store, err := queuestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("create queue store: %v", err)
	}
	ts := httptest.NewServer(queueserver.New(store))
	t.Cleanup(ts.Close)
	return ts.URL + "/devstoreaccount1"
}

// newTableServer spins up an in-process Table emulator and returns its base URL
// (including the development account segment) for raw HTTP requests.
func newTableServer(t *testing.T) string {
	t.Helper()
	store, err := tablestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("create table store: %v", err)
	}
	ts := httptest.NewServer(tableserver.New(store))
	t.Cleanup(ts.Close)
	return ts.URL + "/devstoreaccount1"
}

// azureXMLError is the on-the-wire Azure Storage error body.
type azureXMLError struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

// TestQueueGetMessagesNumOfMessagesUpperBound verifies that numofmessages above
// Azure's maximum of 32 is rejected with 400 OutOfRangeQueryParameterValue,
// while 32 is accepted.
func TestQueueGetMessagesNumOfMessagesUpperBound(t *testing.T) {
	base := newQueueServer(t)

	// Create the queue.
	if resp := doRequest(t, http.MethodPut, base+"/work", ""); resp != http.StatusCreated {
		t.Fatalf("create queue: status %d", resp)
	}

	// numofmessages=33 must be rejected.
	req, err := http.NewRequest(http.MethodGet, base+"/work/messages?numofmessages=33", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("numofmessages=33: status = %d, want %d", res.StatusCode, http.StatusBadRequest)
	}
	body, _ := io.ReadAll(res.Body)
	var azErr azureXMLError
	if err := xml.Unmarshal(body, &azErr); err != nil {
		t.Fatalf("decode error body %q: %v", body, err)
	}
	if azErr.Code != "OutOfRangeQueryParameterValue" {
		t.Fatalf("numofmessages=33: error code = %q, want %q", azErr.Code, "OutOfRangeQueryParameterValue")
	}
	if got := res.Header.Get("x-ms-error-code"); got != "OutOfRangeQueryParameterValue" {
		t.Fatalf("numofmessages=33: x-ms-error-code = %q, want %q", got, "OutOfRangeQueryParameterValue")
	}

	// numofmessages=32 must be accepted.
	if status := doRequest(t, http.MethodGet, base+"/work/messages?numofmessages=32", ""); status != http.StatusOK {
		t.Fatalf("numofmessages=32: status = %d, want %d", status, http.StatusOK)
	}
}

// TestTableListTopBoundary verifies $top is enforced before appending, so
// $top=0 returns zero entities and $top=N returns at most N.
func TestTableListTopBoundary(t *testing.T) {
	base := newTableServer(t)

	// Create the table.
	createTable(t, base, "items")

	// Insert three entities.
	for _, rk := range []string{"a", "b", "c"} {
		insertEntity(t, base, "items", "p", rk)
	}

	if n := listTopCount(t, base, "items", 0); n != 0 {
		t.Fatalf("$top=0 returned %d entities, want 0", n)
	}
	if n := listTopCount(t, base, "items", 2); n != 2 {
		t.Fatalf("$top=2 returned %d entities, want 2", n)
	}
}

// TestTableETagMatchesTimestampSevenDigits verifies that an entity ETag carries
// a fixed seven-digit fractional component consistent with the Timestamp, even
// when the timestamp falls on a whole second (no dropped trailing zeros).
func TestTableETagMatchesTimestampSevenDigits(t *testing.T) {
	base := newTableServer(t)
	createTable(t, base, "items")

	// Insert and read back; retry until we land an entity whose timestamp has a
	// whole-second (or otherwise trailing-zero) fractional part, which is the
	// case that previously dropped zeros in the ETag.
	var props map[string]json.RawMessage
	for i := 0; i < 200; i++ {
		rk := "r" + itoa(i)
		insertEntity(t, base, "items", "p", rk)
		props = getEntity(t, base, "items", "p", rk)
		ts := stringField(t, props, "Timestamp")
		// The Timestamp is rendered with the fixed seven-digit layout. We want
		// a value whose ETag-relevant fractional part exposes trailing zeros.
		if strings.HasSuffix(ts, "0Z") {
			break
		}
	}

	timestamp := stringField(t, props, "Timestamp")
	etag := stringField(t, props, "odata.etag")

	// The ETag embeds a URL-encoded datetime'...' literal.
	decoded, err := url.QueryUnescape(etag)
	if err != nil {
		t.Fatalf("unescape etag %q: %v", etag, err)
	}
	// Extract the datetime literal between the single quotes.
	start := strings.Index(decoded, "datetime'")
	if start < 0 {
		t.Fatalf("etag %q has no datetime literal", decoded)
	}
	rest := decoded[start+len("datetime'"):]
	end := strings.IndexByte(rest, '\'')
	if end < 0 {
		t.Fatalf("etag %q has unterminated datetime literal", decoded)
	}
	etagTime := rest[:end]

	// The fractional second component must be exactly seven digits.
	frac := func(s string) string {
		dot := strings.LastIndexByte(s, '.')
		if dot < 0 {
			return ""
		}
		zEnd := strings.IndexByte(s[dot:], 'Z')
		if zEnd < 0 {
			return s[dot+1:]
		}
		return s[dot+1 : dot+zEnd]
	}
	etagFrac := frac(etagTime)
	tsFrac := frac(timestamp)
	if len(etagFrac) != 7 {
		t.Fatalf("etag fractional digits = %q (len %d), want 7-digit form (timestamp=%q etag=%q)",
			etagFrac, len(etagFrac), timestamp, etagTime)
	}
	if etagFrac != tsFrac {
		t.Fatalf("etag fractional %q != timestamp fractional %q (etag=%q timestamp=%q)",
			etagFrac, tsFrac, etagTime, timestamp)
	}
}

// --- raw-HTTP helpers for the fidelity tests ---

func doRequest(t *testing.T, method, urlStr, body string) int {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, urlStr, rdr)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, urlStr, err)
	}
	defer res.Body.Close()
	_, _ = io.ReadAll(res.Body)
	return res.StatusCode
}

func createTable(t *testing.T, base, name string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"TableName": name})
	req, err := http.NewRequest(http.MethodPost, base+"/Tables", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("build create-table request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json;odata=nometadata")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	defer res.Body.Close()
	_, _ = io.ReadAll(res.Body)
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusNoContent {
		t.Fatalf("create table: status %d", res.StatusCode)
	}
}

func insertEntity(t *testing.T, base, table, pk, rk string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"PartitionKey": pk, "RowKey": rk})
	req, err := http.NewRequest(http.MethodPost, base+"/"+table, strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("build insert request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json;odata=nometadata")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("insert entity: %v", err)
	}
	defer res.Body.Close()
	_, _ = io.ReadAll(res.Body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("insert entity: status %d", res.StatusCode)
	}
}

func getEntity(t *testing.T, base, table, pk, rk string) map[string]json.RawMessage {
	t.Helper()
	u := base + "/" + table + "(PartitionKey='" + pk + "',RowKey='" + rk + "')"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		t.Fatalf("build get request: %v", err)
	}
	req.Header.Set("Accept", "application/json;odata=nometadata")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get entity: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get entity: status %d body %s", res.StatusCode, body)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode entity %s: %v", body, err)
	}
	return out
}

func listTopCount(t *testing.T, base, table string, top int) int {
	t.Helper()
	u := base + "/" + table + "?$top=" + itoa(top)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		t.Fatalf("build list request: %v", err)
	}
	req.Header.Set("Accept", "application/json;odata=nometadata")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list entities: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list entities: status %d body %s", res.StatusCode, body)
	}
	var out struct {
		Value []map[string]json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode list %s: %v", body, err)
	}
	return len(out.Value)
}

func stringField(t *testing.T, m map[string]json.RawMessage, key string) string {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("entity missing %q field", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("decode %q (%s): %v", key, raw, err)
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
