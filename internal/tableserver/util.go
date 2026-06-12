package tableserver

import (
	"errors"
	"time"
)

// timeFmt is the RFC1123 GMT layout for the Date response header.
const timeFmt = "Mon, 02 Jan 2006 15:04:05 GMT"

func nowHTTP() string {
	return time.Now().UTC().Format(timeFmt)
}

func isErr(err, target error) bool {
	return errors.Is(err, target)
}

// preferNoContent reports whether the client asked for a no-content response.
func preferNoContent(prefer string) bool {
	return prefer == "return-no-content"
}
