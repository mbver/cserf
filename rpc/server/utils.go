package server

import (
	"io"

	"golang.org/x/crypto/bcrypt"
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

func HashPwd(pwd string) (string, error) {
	hbytes, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hbytes), nil
}

func ComparePwdAndHash(pwd, hash string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)); err != nil {
		return false
	}
	return true
}
