package ssh

import (
	"context"
	"testing"

	"sshm/internal/repository"
)

func TestDialClearsCredentialOnAuthenticationError(t *testing.T) {
	credential := []byte("secret")
	_, err := dial(context.Background(), repository.Connection{Credential: "unsupported"}, credential)
	if err == nil {
		t.Fatal("不支持的认证类型未返回错误")
	}
	for _, value := range credential {
		if value != 0 {
			t.Fatalf("认证失败后明文凭据未清空: %v", credential)
		}
	}
}
