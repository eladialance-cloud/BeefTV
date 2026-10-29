package storageconfig

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestNewS3ClientUsesCompatibleChecksumModes(t *testing.T) {
	client := newS3Client(Target{
		Provider: "cos", Endpoint: "https://cos.ap-shanghai.myqcloud.com", Region: "ap-shanghai",
		Bucket: "example-1250000000", AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "secret",
	}, time.Second)
	options := client.Options()
	if options.RequestChecksumCalculation != aws.RequestChecksumCalculationWhenRequired {
		t.Fatalf("request checksum mode = %d", options.RequestChecksumCalculation)
	}
	if options.ResponseChecksumValidation != aws.ResponseChecksumValidationWhenRequired {
		t.Fatalf("response checksum mode = %d", options.ResponseChecksumValidation)
	}
}
