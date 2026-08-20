package crypto

import "os"

func Init() error { return nil }

func GetEncryptedEnv(key string) (string, error) {
	return os.Getenv(key), nil
}
