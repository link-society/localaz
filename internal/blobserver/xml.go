package blobserver

import (
	"encoding/xml"
	"io"
	"strings"

	"localaz.dev/internal/blobstore"
)

// timeFmt is the RFC1123 GMT layout Azure uses for Last-Modified values.
const timeFmt = "Mon, 02 Jan 2006 15:04:05 GMT"

// --- List Containers ---

type enumContainersResult struct {
	XMLName         xml.Name      `xml:"EnumerationResults"`
	ServiceEndpoint string        `xml:"ServiceEndpoint,attr"`
	Prefix          string        `xml:"Prefix,omitempty"`
	Containers      xmlContainers `xml:"Containers"`
	NextMarker      string        `xml:"NextMarker"`
}

type xmlContainers struct {
	Items []xmlContainer `xml:"Container"`
}

type xmlContainer struct {
	Name       string            `xml:"Name"`
	Properties xmlContainerProps `xml:"Properties"`
}

type xmlContainerProps struct {
	LastModified string `xml:"Last-Modified"`
	ETag         string `xml:"Etag"`
	LeaseStatus  string `xml:"LeaseStatus"`
	LeaseState   string `xml:"LeaseState"`
}

func marshalContainers(endpoint, prefix string, items []blobstore.ContainerInfo) ([]byte, error) {
	res := enumContainersResult{ServiceEndpoint: endpoint, Prefix: prefix}
	for _, c := range items {
		res.Containers.Items = append(res.Containers.Items, xmlContainer{
			Name: c.Name,
			Properties: xmlContainerProps{
				LastModified: c.LastModified.UTC().Format(timeFmt),
				ETag:         c.ETag,
				LeaseStatus:  "unlocked",
				LeaseState:   "available",
			},
		})
	}
	return marshalDoc(res)
}

// --- List Blobs ---

type enumBlobsResult struct {
	XMLName         xml.Name `xml:"EnumerationResults"`
	ServiceEndpoint string   `xml:"ServiceEndpoint,attr"`
	ContainerName   string   `xml:"ContainerName,attr"`
	Prefix          string   `xml:"Prefix"`
	Marker          string   `xml:"Marker"`
	MaxResults      int      `xml:"MaxResults,omitempty"`
	Delimiter       string   `xml:"Delimiter,omitempty"`
	Blobs           xmlBlobs `xml:"Blobs"`
	NextMarker      string   `xml:"NextMarker"`
}

type xmlBlobs struct {
	Blobs    []xmlBlob       `xml:"Blob"`
	Prefixes []xmlBlobPrefix `xml:"BlobPrefix"`
}

type xmlBlob struct {
	Name       string       `xml:"Name"`
	Properties xmlBlobProps `xml:"Properties"`
}

type xmlBlobPrefix struct {
	Name string `xml:"Name"`
}

type xmlBlobProps struct {
	LastModified    string `xml:"Last-Modified"`
	ETag            string `xml:"Etag"`
	ContentLength   int64  `xml:"Content-Length"`
	ContentType     string `xml:"Content-Type"`
	ContentMD5      string `xml:"Content-MD5,omitempty"`
	BlobType        string `xml:"BlobType"`
	LeaseStatus     string `xml:"LeaseStatus"`
	LeaseState      string `xml:"LeaseState"`
	ServerEncrypted bool   `xml:"ServerEncrypted"`
}

func marshalBlobs(endpoint, containerName, prefix, delimiter string, blobs []blobstore.BlobInfo, prefixes []string) ([]byte, error) {
	res := enumBlobsResult{
		ServiceEndpoint: endpoint,
		ContainerName:   containerName,
		Prefix:          prefix,
		Delimiter:       delimiter,
	}
	for _, b := range blobs {
		ct := b.Props.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		res.Blobs.Blobs = append(res.Blobs.Blobs, xmlBlob{
			Name: b.Name,
			Properties: xmlBlobProps{
				LastModified:    b.LastModified.UTC().Format(timeFmt),
				ETag:            b.ETag,
				ContentLength:   b.ContentLength,
				ContentType:     ct,
				BlobType:        b.BlobType,
				LeaseStatus:     "unlocked",
				LeaseState:      "available",
				ServerEncrypted: true,
			},
		})
	}
	for _, p := range prefixes {
		res.Blobs.Prefixes = append(res.Blobs.Prefixes, xmlBlobPrefix{Name: p})
	}
	return marshalDoc(res)
}

func marshalDoc(v any) ([]byte, error) {
	body, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

// parseBlockList reads a Put Block List request body and returns the referenced
// block IDs in document order. Latest, Committed and Uncommitted are treated
// identically because the emulator only tracks staged blocks.
func parseBlockList(r io.Reader) ([]string, error) {
	dec := xml.NewDecoder(r)
	var ids []string
	var capture bool
	var current string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "Latest", "Committed", "Uncommitted":
				capture = true
				current = ""
			}
		case xml.CharData:
			if capture {
				current += string(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "Latest", "Committed", "Uncommitted":
				ids = append(ids, strings.TrimSpace(current))
				capture = false
			}
		}
	}
	return ids, nil
}
