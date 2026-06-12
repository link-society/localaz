package blobserver

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"localaz.dev/internal/stores/blobstore"
	"localaz.dev/internal/utils/azerr"
	"localaz.dev/internal/utils/azwire"
)

const maxBlobBytes = 5 << 30 // 5 GiB upload guard for the single-shot path.

func readBlobProps(h http.Header) blobstore.BlobProps {
	// The single-shot Put Blob path carries the blob content type in the request
	// Content-Type header; the staged Put Block List path uses the explicit
	// x-ms-blob-content-type header. Prefer the latter when present.
	contentType := h.Get("x-ms-blob-content-type")
	if contentType == "" {
		contentType = h.Get("Content-Type")
	}
	return blobstore.BlobProps{
		ContentType:        contentType,
		ContentEncoding:    h.Get("x-ms-blob-content-encoding"),
		ContentLanguage:    h.Get("x-ms-blob-content-language"),
		ContentDisposition: h.Get("x-ms-blob-content-disposition"),
		CacheControl:       h.Get("x-ms-blob-cache-control"),
		Metadata:           readMetadata(h),
	}
}

func (s *Server) putBlob(w http.ResponseWriter, r *http.Request, req request) {
	if bt := r.Header.Get("x-ms-blob-type"); bt != "" && bt != "BlockBlob" {
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeUnsupportedHeader,
			fmt.Sprintf("Blob type %q is not supported by the emulator.", bt)))
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxBlobBytes))
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	props := readBlobProps(r.Header)
	sum := md5.Sum(data)
	props.ContentMD5 = sum[:]

	info, err := s.store.PutBlob(req.account, req.container, req.blob, data, props)
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", azwire.FormatHTTP(info.LastModified))
	w.Header().Set("Content-MD5", base64MD5(info.Props.ContentMD5))
	w.Header().Set("x-ms-request-server-encrypted", "true")
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) getBlob(w http.ResponseWriter, r *http.Request, req request, headOnly bool) {
	data, info, err := s.store.GetBlob(req.account, req.container, req.blob)
	if s.mapStoreErr(w, r, err) {
		return
	}

	ct := info.Props.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("ETag", info.ETag)
	h.Set("Last-Modified", azwire.FormatHTTP(info.LastModified))
	h.Set("x-ms-blob-type", info.BlobType)
	h.Set("x-ms-creation-time", azwire.FormatHTTP(info.LastModified))
	h.Set("x-ms-server-encrypted", "true")
	h.Set("Accept-Ranges", "bytes")
	if len(info.Props.ContentMD5) > 0 {
		h.Set("Content-MD5", base64MD5(info.Props.ContentMD5))
	}
	if info.Props.ContentEncoding != "" {
		h.Set("Content-Encoding", info.Props.ContentEncoding)
	}
	if info.Props.ContentLanguage != "" {
		h.Set("Content-Language", info.Props.ContentLanguage)
	}
	if info.Props.CacheControl != "" {
		h.Set("Cache-Control", info.Props.CacheControl)
	}
	if info.Props.ContentDisposition != "" {
		h.Set("Content-Disposition", info.Props.ContentDisposition)
	}
	writeMetadata(h, info.Props.Metadata)

	// Range support (Range or x-ms-range), single range only.
	rangeHeader := r.Header.Get("x-ms-range")
	if rangeHeader == "" {
		rangeHeader = r.Header.Get("Range")
	}
	start, end, ok := parseRange(rangeHeader, int64(len(data)))
	if ok {
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		h.Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent)
		if !headOnly {
			_, _ = w.Write(data[start : end+1])
		}
		return
	}

	h.Set("Content-Length", strconv.FormatInt(info.ContentLength, 10))
	w.WriteHeader(http.StatusOK)
	if !headOnly {
		_, _ = w.Write(data)
	}
}

func (s *Server) deleteBlob(w http.ResponseWriter, r *http.Request, req request) {
	err := s.store.DeleteBlob(req.account, req.container, req.blob)
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) putBlock(w http.ResponseWriter, r *http.Request, req request, q queryValues) {
	blockID := q.Get("blockid")
	if blockID == "" {
		s.writeError(w, r, azerr.New(http.StatusBadRequest, azerr.CodeInvalidQueryParameter,
			"blockid is required for Put Block."))
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxBlobBytes))
	if err != nil {
		s.writeError(w, r, azerr.Internal(err.Error()))
		return
	}
	if err := s.store.StageBlock(req.account, req.container, req.blob, blockID, data); err != nil {
		if s.mapStoreErr(w, r, err) {
			return
		}
	}
	sum := md5.Sum(data)
	w.Header().Set("Content-MD5", base64MD5(sum[:]))
	w.Header().Set("x-ms-request-server-encrypted", "true")
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) putBlockList(w http.ResponseWriter, r *http.Request, req request) {
	ids, err := parseBlockList(r.Body)
	if err != nil {
		s.writeError(w, r, azerr.InvalidBlockList())
		return
	}
	props := readBlobProps(r.Header)
	info, err := s.store.CommitBlockList(req.account, req.container, req.blob, ids, props)
	if s.mapStoreErr(w, r, err) {
		return
	}
	w.Header().Set("ETag", info.ETag)
	w.Header().Set("Last-Modified", azwire.FormatHTTP(info.LastModified))
	w.Header().Set("x-ms-request-server-encrypted", "true")
	w.WriteHeader(http.StatusCreated)
}

func base64MD5(sum []byte) string {
	if len(sum) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(sum)
}

// parseRange parses a single-range "bytes=start-end" specification. It returns
// the inclusive bounds and whether a usable range was found.
func parseRange(spec string, size int64) (int64, int64, bool) {
	if spec == "" || size == 0 {
		return 0, 0, false
	}
	spec = strings.TrimSpace(spec)
	if !strings.HasPrefix(spec, "bytes=") {
		return 0, 0, false
	}
	spec = strings.TrimPrefix(spec, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, false // multi-range not supported
	}
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, false
	}
	startStr, endStr := spec[:dash], spec[dash+1:]
	var start, end int64
	var err error
	if startStr == "" {
		// suffix range: last N bytes
		n, e := strconv.ParseInt(endStr, 10, 64)
		if e != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	}
	start, err = strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	if endStr == "" {
		end = size - 1
	} else {
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return 0, 0, false
		}
	}
	if end >= size {
		end = size - 1
	}
	if end < start {
		return 0, 0, false
	}
	return start, end, true
}
