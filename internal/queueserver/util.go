package queueserver

import (
	"errors"
	"time"
)

func nowHTTP() string {
	return time.Now().UTC().Format(timeFmt)
}

func isErr(err, target error) bool {
	return errors.Is(err, target)
}
