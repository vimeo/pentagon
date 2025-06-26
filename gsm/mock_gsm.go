package gsm

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

type MockGSM struct{}

func NewMockGSM() *MockGSM {
	return &MockGSM{}
}

func (m *MockGSM) AccessSecretVersion(
	ctx context.Context,
	req *secretmanagerpb.AccessSecretVersionRequest,
	opts ...gax.CallOption,
) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	fields := strings.Split(req.Name, "/")
	var data string
	if strings.Contains(req.Name, "locations") {
		if len(fields) != 8 {
			return nil, errors.New("invalid regional secret path")
		}
		// project, location, secret, version
		data = fmt.Sprintf("%s_%s_%s_%s", fields[1], fields[3], fields[5], fields[7])
	} else {
		if len(fields) != 6 {
			return nil, errors.New("invalid secret path")
		}
		// project, secret, version
		data = fmt.Sprintf("%s_%s_%s", fields[1], fields[3], fields[5])
	}

	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(data),
		},
	}, nil
}
