package flyetcd

import (
	"crypto/md5"
	"encoding/hex"
)

func getMD5Hash(str string) string {
	hasher := md5.New()
	hasher.Write([]byte(str))
	return hex.EncodeToString(hasher.Sum(nil))
}
