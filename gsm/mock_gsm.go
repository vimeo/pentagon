package gsm

import (
	"context"
	"fmt"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
)

// SecretAccessor exposes the AccessSecretVersion method from the SecretManager Client.
type SecretAccessor interface {
	AccessSecretVersion(
		context.Context,
		*secretmanagerpb.AccessSecretVersionRequest,
		...gax.CallOption,
	) (*secretmanagerpb.AccessSecretVersionResponse, error)
}

type MockGSM struct {
	Data map[string][]byte
}

func NewMockGSM(data map[string][]byte) *MockGSM {
	return &MockGSM{
		Data: data,
	}
}

func (m *MockGSM) AccessSecretVersion(
	ctx context.Context,
	req *secretmanagerpb.AccessSecretVersionRequest,
	opts ...gax.CallOption,
) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	secretVal, ok := m.Data[req.Name]
	if !ok {
		return nil, fmt.Errorf("secret %q not found", req.Name)
	}

	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{
			Data: secretVal,
		},
	}, nil
}
