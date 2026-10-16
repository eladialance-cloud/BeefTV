package payment

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strings"

	"infinite-canvas/backend/internal/kernel"
)

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil {
		return nil, errors.New("private key PEM parse failed")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		return key, nil
	default:
		return nil, errors.New("unsupported private key PEM type")
	}
}

func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil {
		return nil, errors.New("public key PEM parse failed")
	}
	switch block.Type {
	case "PUBLIC KEY":
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		key, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("public key is not RSA")
		}
		return key, nil
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case "CERTIFICATE":
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		key, ok := certificate.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("certificate public key is not RSA")
		}
		return key, nil
	default:
		return nil, errors.New("unsupported public key PEM type")
	}
}

func signRSASHA256(key *rsa.PrivateKey, message string) (string, error) {
	if key == nil {
		return "", errors.New("RSA private key is nil")
	}
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyRSASHA256(key *rsa.PublicKey, message string, signature string) error {
	if key == nil {
		return errors.New("RSA public key is nil")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], decoded)
}

func invalidGatewayConfig(reason string) error {
	return kernel.BadAuthRequest("支付渠道配置不完整：" + reason)
}
