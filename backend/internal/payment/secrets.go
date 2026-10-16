package payment

import (
	"encoding/json"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/localcrypto"
)

// encryptSecrets serialises the credential bundle and wraps it in the shared
// AES-GCM format. An all-empty bundle stores nothing rather than an encrypted
// empty object, so "no secret configured" stays a cheap check.
func encryptSecrets(dataDir string, value secrets) (string, error) {
	if value.APIKey == "" && value.PrivateKey == "" && value.PlatformPublicKey == "" {
		return "", nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", kernel.WrapAppError(kernel.CodeInternal, "支付渠道密钥序列化失败", err)
	}
	encrypted, err := localcrypto.Encrypt(dataDir, string(payload))
	if err != nil {
		return "", kernel.WrapAppError(kernel.CodeInternal, "支付渠道密钥加密失败", err)
	}
	return encrypted, nil
}

func decodeSecrets(plain string, target *secrets) error {
	if plain == "" {
		return nil
	}
	return json.Unmarshal([]byte(plain), target)
}
