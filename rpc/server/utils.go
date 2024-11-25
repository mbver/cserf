package server

import (
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func CombineCleanup(cleanups ...func()) func() {
	return func() {
		for _, f := range cleanups {
			f()
		}
	}
}

func ShouldStopStreaming(err error) bool {
	st, ok := status.FromError(err)
	if ok {
		if st.Code() == codes.Unavailable {
			return true
		}
	}
	if err == io.EOF {
		return true
	}
	return false
}
