package config

import (
	"github.com/zalando/go-keyring"
)

type Config struct {
	MinimizeOnClose bool
	EnterpriseMode  bool
	PassKey         string
	SafeMode        bool
}

func GetKey() string {
	secret, err := keyring.Get("Wipr_verify", "Wipr_user")
	if err == nil {
		return secret
	}
	return ""
}

func SetKey(s string) {
	keyring.Set("Wipr_verify", "Wipr_user", s)
}

func DeleteKey() {
	keyring.Delete("Wipr_verify", "Wipr_user")
}
