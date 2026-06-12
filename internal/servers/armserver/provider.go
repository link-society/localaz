package armserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"localaz.dev/internal/utils/httpx"
)

// handleProvider serves resource-provider requests such as
// .../providers/Microsoft.ServiceBus/namespaces/{ns}. It stores whatever the
// client sends as a generic resource and echoes it back with a synthesized
// terminal provisioningState, which is enough for the Azure CLI's management
// commands (create/show/list/delete) to drive the emulator end to end.
func (s *Server) handleProvider(w http.ResponseWriter, r *http.Request, segments []string) {
	pi := providerIndex(segments)
	provider := ""
	if pi+1 < len(segments) {
		provider = segments[pi+1]
	}
	rest := segments[pi+2:]

	// Provider-scoped POST actions.
	if r.Method == http.MethodPost && len(rest) > 0 {
		switch strings.ToLower(rest[len(rest)-1]) {
		case "checknameavailability":
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"nameAvailable": true, "reason": "None"})
			return
		case "listkeys":
			s.writeListKeys(w, segments, pi)
			return
		}
	}

	// Provider metadata: .../providers/Microsoft.ServiceBus (registration state).
	if len(rest) == 0 {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"id":                "/" + strings.Join(segments, "/"),
			"namespace":         provider,
			"registrationState": "Registered",
		})
		return
	}

	resourceType := providerType(provider, rest)

	// Odd remainder ⇒ a collection (ends in a type name); even ⇒ a single item.
	if len(rest)%2 == 1 {
		s.handleProviderCollection(w, r, segments, resourceType)
		return
	}
	s.handleProviderItem(w, r, segments, resourceType, rest[len(rest)-1])
}

// providerType builds the ARM resource type from the provider and the trailing
// type/name pairs, e.g. ("Microsoft.ServiceBus", [namespaces ns queues q]) ⇒
// "Microsoft.ServiceBus/namespaces/queues".
func providerType(provider string, rest []string) string {
	parts := []string{provider}
	for i := 0; i < len(rest); i += 2 {
		parts = append(parts, rest[i])
	}
	return strings.Join(parts, "/")
}

func (s *Server) handleProviderItem(w http.ResponseWriter, r *http.Request, segments []string, resourceType, name string) {
	id := "/" + strings.Join(segments, "/")
	switch r.Method {
	case http.MethodPut:
		body := map[string]any{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		body["id"] = id
		body["name"] = name
		body["type"] = resourceType

		props, _ := body["properties"].(map[string]any)
		if props == nil {
			props = map[string]any{}
		}
		props["provisioningState"] = "Succeeded"
		if strings.EqualFold(resourceType, "Microsoft.ServiceBus/namespaces") {
			props["serviceBusEndpoint"] = "sb://" + name + "." + s.store.Config().StorageSuffix + "/"
			props["status"] = "Active"
		}
		body["properties"] = props

		s.store.PutResource(id, body)
		httpx.WriteJSON(w, http.StatusOK, body)
	case http.MethodGet:
		res, ok := s.store.GetResource(id)
		if !ok {
			httpx.WriteError(w, http.StatusNotFound, "ResourceNotFound", "The resource '"+name+"' could not be found.")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, res)
	case http.MethodDelete:
		s.store.DeleteResource(id)
		w.WriteHeader(http.StatusOK)
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
	}
}

func (s *Server) handleProviderCollection(w http.ResponseWriter, r *http.Request, segments []string, resourceType string) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
		return
	}
	// Resource-group-scoped collections match by the collection path prefix;
	// subscription-scoped collections match every resource of the type in the
	// subscription (their IDs carry a /resourceGroups/ segment).
	prefix := "/" + strings.Join(segments, "/") + "/"
	if !containsFold(segments, "resourcegroups") && len(segments) >= 2 {
		prefix = "/" + segments[0] + "/" + segments[1]
	}
	httpx.WriteJSON(w, http.StatusOK, listEnvelope{Value: s.store.ListResources(prefix, resourceType)})
}

// writeListKeys returns a synthetic key set and connection strings for a
// namespace authorization rule.
func (s *Server) writeListKeys(w http.ResponseWriter, segments []string, pi int) {
	ns := ""
	rest := segments[pi+2:]
	if len(rest) >= 2 && strings.EqualFold(rest[0], "namespaces") {
		ns = rest[1]
	}
	endpoint := "sb://" + ns + "." + s.store.Config().StorageSuffix + "/"
	const key = "localaz-shared-access-key"
	conn := "Endpoint=" + endpoint + ";SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=" + key
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"keyName":                   "RootManageSharedAccessKey",
		"primaryKey":                key,
		"secondaryKey":              key,
		"primaryConnectionString":   conn,
		"secondaryConnectionString": conn,
	})
}

// containsFold reports whether segments contains target (case-insensitive).
func containsFold(segments []string, target string) bool {
	for _, seg := range segments {
		if strings.EqualFold(seg, target) {
			return true
		}
	}
	return false
}
