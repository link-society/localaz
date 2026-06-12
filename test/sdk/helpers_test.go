// Package sdk contains the integration test suite for localaz. Each test drives
// the emulator through an official Azure Go SDK, the strongest guarantee that
// real client applications interoperate with localaz: every request is built,
// signed and parsed by the same code a production application would use.
//
// Test files in this package contain only test cases. Every shared helper —
// cross-service utilities and per-service setup alike — lives in this file.
package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/eventgrid/aznamespaces"
	azingest "github.com/Azure/azure-sdk-for-go/sdk/monitor/ingestion/azlogs"
	azquery "github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azlogs"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
	"github.com/gorilla/websocket"

	"localaz.dev/internal/servers/aadserver"
	"localaz.dev/internal/servers/armserver"
	"localaz.dev/internal/servers/blobserver"
	"localaz.dev/internal/servers/egserver"
	"localaz.dev/internal/servers/monitorserver"
	"localaz.dev/internal/servers/queueserver"
	"localaz.dev/internal/servers/sbserver"
	"localaz.dev/internal/servers/tableserver"
	"localaz.dev/internal/stores/armstore"
	"localaz.dev/internal/stores/blobstore/fsstore"
	"localaz.dev/internal/stores/egstore"
	"localaz.dev/internal/stores/monitorstore"
	"localaz.dev/internal/stores/queuestore"
	"localaz.dev/internal/stores/sbstore"
	"localaz.dev/internal/stores/tablestore"
)

// --- Cross-service helpers ---

// ctx returns a context for a test request.
func ctx(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

// ptr returns a pointer to s, for the many SDK fields typed as *string.
func ptr(s string) *string { return &s }

// metadataValue looks up a metadata key case-insensitively, returning the empty
// string if it is absent. Azure treats metadata keys case-insensitively and
// HTTP header canonicalization may alter the case returned to the client.
func metadataValue(m map[string]*string, key string) string {
	for k, v := range m {
		if strings.EqualFold(k, key) && v != nil {
			return *v
		}
	}
	return ""
}

// fakeCredential satisfies azcore.TokenCredential. The emulator never validates
// bearer tokens, so any non-empty token works.
type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "localaz-dev-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

// --- Entra ID (AAD) ---

// newAAD spins up an in-process Entra ID emulator over TLS (azidentity refuses
// to send credentials over plain HTTP) and returns the test server.
func newAAD(t *testing.T) *httptest.Server {
	t.Helper()
	srv, err := aadserver.New()
	if err != nil {
		t.Fatalf("create aad server: %v", err)
	}
	ts := httptest.NewTLSServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

// decodeJWTClaims decodes the payload segment of a compact JWT.
func decodeJWTClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not a compact JWT: %d segments", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode token payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal token payload: %v", err)
	}
	return claims
}

// --- Resource Manager (ARM) ---

const testSubscriptionID = "00000000-0000-0000-0000-000000000000"

// newARM spins up an in-process Resource Manager emulator over TLS and returns
// the test server together with client options wired to it.
func newARM(t *testing.T) (*httptest.Server, *arm.ClientOptions) {
	t.Helper()
	store := armstore.New(armstore.Config{})
	ts := httptest.NewTLSServer(armserver.New(store))
	t.Cleanup(ts.Close)

	opts := &arm.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: cloud.Configuration{
				Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
					cloud.ResourceManager: {Endpoint: ts.URL, Audience: ts.URL},
				},
			},
			Transport: ts.Client(),
		},
	}
	return ts, opts
}

// --- Blob storage ---

// newClient spins up an in-process emulator backed by a temporary data
// directory and returns an azblob client pointed at it.
func newClient(t *testing.T) *azblob.Client {
	t.Helper()
	store, err := fsstore.New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	ts := httptest.NewServer(blobserver.New(store))
	t.Cleanup(ts.Close)

	client, err := azblob.NewClientWithNoCredential(ts.URL+"/devstoreaccount1", nil)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

// --- Queue storage ---

// newQueueClient spins up an in-process Queue emulator and returns a service
// client pointed at it.
func newQueueClient(t *testing.T) *azqueue.ServiceClient {
	t.Helper()
	store, err := queuestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	ts := httptest.NewServer(queueserver.New(store))
	t.Cleanup(ts.Close)

	client, err := azqueue.NewServiceClientWithNoCredential(ts.URL+"/devstoreaccount1", nil)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

// --- Table storage ---

// newTableServiceClient spins up an in-process Table emulator and returns a
// service client pointed at it.
func newTableServiceClient(t *testing.T) *aztables.ServiceClient {
	t.Helper()
	store, err := tablestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	ts := httptest.NewServer(tableserver.New(store))
	t.Cleanup(ts.Close)

	client, err := aztables.NewServiceClientWithNoCredential(ts.URL+"/devstoreaccount1", nil)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return client
}

// tableEntity is a minimal entity shape for marshalling test payloads.
type tableEntity struct {
	PartitionKey string `json:"PartitionKey"`
	RowKey       string `json:"RowKey"`
	Name         string `json:"Name,omitempty"`
	Count        int    `json:"Count,omitempty"`
}

func marshalEntity(t *testing.T, e tableEntity) []byte {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal entity: %v", err)
	}
	return b
}

// --- Event Grid ---

// newEventGrid spins up an in-process Event Grid emulator and returns sender and
// receiver clients pointed at the given topic / subscription.
func newEventGrid(t *testing.T, topic, subscription string) (*aznamespaces.SenderClient, *aznamespaces.ReceiverClient) {
	t.Helper()
	// The Event Grid SDK refuses to send shared-key credentials over plain HTTP,
	// so the emulator is exposed over TLS and the SDK is pointed at the test
	// server's trusting client.
	ts := httptest.NewTLSServer(egserver.New(egstore.New()))
	t.Cleanup(ts.Close)

	cred := azcore.NewKeyCredential("localaz-dev-key")

	sendOpts := &aznamespaces.SenderClientOptions{}
	sendOpts.Transport = ts.Client()
	sender, err := aznamespaces.NewSenderClientWithSharedKeyCredential(ts.URL, topic, cred, sendOpts)
	if err != nil {
		t.Fatalf("create sender: %v", err)
	}

	recvOpts := &aznamespaces.ReceiverClientOptions{}
	recvOpts.Transport = ts.Client()
	receiver, err := aznamespaces.NewReceiverClientWithSharedKeyCredential(ts.URL, topic, subscription, cred, recvOpts)
	if err != nil {
		t.Fatalf("create receiver: %v", err)
	}
	return sender, receiver
}

// --- Monitor Logs ---

// newMonitor spins up an in-process Monitor Logs emulator and returns ingestion
// and query clients pointed at it.
func newMonitor(t *testing.T) (*azingest.Client, *azquery.Client) {
	t.Helper()
	// The Monitor SDKs send bearer credentials, which azcore refuses to do over
	// plain HTTP, so the emulator is exposed over TLS and the SDK is pointed at
	// the test server's trusting client.
	ts := httptest.NewTLSServer(monitorserver.New(monitorstore.New()))
	t.Cleanup(ts.Close)

	cfg := cloud.Configuration{
		Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
			azingest.ServiceNameIngestion: {Audience: "https://monitor.azure.com", Endpoint: ts.URL},
			azquery.ServiceName:           {Audience: "https://api.loganalytics.io", Endpoint: ts.URL},
		},
	}

	ingOpts := &azingest.ClientOptions{}
	ingOpts.Cloud = cfg
	ingOpts.Transport = ts.Client()
	ingest, err := azingest.NewClient(ts.URL, fakeCredential{}, ingOpts)
	if err != nil {
		t.Fatalf("create ingestion client: %v", err)
	}

	qOpts := &azquery.ClientOptions{}
	qOpts.Cloud = cfg
	qOpts.Transport = ts.Client()
	query, err := azquery.NewClient(fakeCredential{}, qOpts)
	if err != nil {
		t.Fatalf("create query client: %v", err)
	}
	return ingest, query
}

// upload marshals records to JSON and uploads them to the given stream.
func upload(t *testing.T, c *azingest.Client, stream string, records []map[string]any) {
	t.Helper()
	body, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal records: %v", err)
	}
	if _, err := c.Upload(ctx(t), "dcr-localaz", stream, body, nil); err != nil {
		t.Fatalf("upload logs: %v", err)
	}
}

// queryRows runs a KQL query and returns the single PrimaryResult table.
func queryRows(t *testing.T, c *azquery.Client, kql string) azquery.Table {
	t.Helper()
	resp, err := c.QueryWorkspace(ctx(t), "workspace-localaz", azquery.QueryBody{Query: to.Ptr(kql)}, nil)
	if err != nil {
		t.Fatalf("query workspace: %v", err)
	}
	if len(resp.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(resp.Tables))
	}
	return resp.Tables[0]
}

// --- Service Bus ---

// startServiceBus boots the emulator's AMQP listener on a random local port and
// returns a connection string that uses the development-emulator flag (plain
// TCP + anonymous SASL).
func startServiceBus(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := sbserver.New(sbstore.New())
	go server.Serve(listener)
	t.Cleanup(func() { listener.Close() })

	port := listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf(
		"Endpoint=sb://127.0.0.1:%d;SharedAccessKeyName=test;SharedAccessKey=test;UseDevelopmentEmulator=true",
		port,
	)
}

// --- Web PubSub ---

const wpsSubprotocol = "json.webpubsub.azure.v1"

func readFrame(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	_, raw, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket frame: %v", err)
	}
	var frame map[string]any
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("decode websocket frame: %v", err)
	}
	return frame
}

// waitForType reads frames until one of the given type arrives.
func waitForType(t *testing.T, c *websocket.Conn, msgType string) map[string]any {
	t.Helper()
	for i := 0; i < 10; i++ {
		frame := readFrame(t, c)
		if frame["type"] == msgType {
			return frame
		}
	}
	t.Fatalf("did not receive a %q frame", msgType)
	return nil
}
