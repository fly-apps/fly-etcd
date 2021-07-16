package flyetcd

import (
	"crypto/md5"
	"encoding/hex"
	"os"
)

func getMD5Hash(str string) string {
	hasher := md5.New()
	hasher.Write([]byte(str))
	return hex.EncodeToString(hasher.Sum(nil))
}

func envOrDefault(name, defaultVal string) string {
	val, ok := os.LookupEnv(name)
	if ok {
		return val
	}
	return defaultVal
}
