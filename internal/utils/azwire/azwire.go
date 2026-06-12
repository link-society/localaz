// Package azwire holds helpers for the Azure storage wire format shared by the
// Blob, Queue and Table protocols: the RFC1123 GMT timestamps Azure uses for
// Last-Modified/Date headers and the XML document framing (declaration header)
// the SDKs expect.
package azwire

import (
	"encoding/xml"
	"time"
)

// HTTPTime is the RFC1123 GMT layout Azure uses for Last-Modified and Date.
const HTTPTime = "Mon, 02 Jan 2006 15:04:05 GMT"

// NowHTTP renders the current UTC time in the Azure HTTP date layout.
func NowHTTP() string {
	return time.Now().UTC().Format(HTTPTime)
}

// FormatHTTP renders t (in UTC) in the Azure HTTP date layout.
func FormatHTTP(t time.Time) string {
	return t.UTC().Format(HTTPTime)
}

// MarshalXML serialises v to XML prefixed with the XML declaration, with no
// indentation (the compact form the Queue service emits).
func MarshalXML(v any) ([]byte, error) {
	body, err := xml.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

// MarshalXMLIndent is like MarshalXML but indents the document with two spaces
// (the pretty form the Blob service emits).
func MarshalXMLIndent(v any) ([]byte, error) {
	body, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}
