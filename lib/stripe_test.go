package lib

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/mock"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/form"
	"testing"
)

type StripeMockBackend struct {
	mock.Mock
}

func (s *StripeMockBackend) Call(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	args := s.Called(method, path, key, params, v)
	return args.Error(0)
}

func (s *StripeMockBackend) CallRaw(method, path, key string, body *form.Values, params *stripe.Params, v stripe.LastResponseSetter) error {
	args := s.Called(method, path, key, body, params, v)
	return args.Error(0)
}

func (s *StripeMockBackend) CallMultipart(method, path, key, boundary string, body *bytes.Buffer, params *stripe.Params, v stripe.LastResponseSetter) error {
	args := s.Called(params)
	return args.Error(0)
}

func (s *StripeMockBackend) SetMaxNetworkRetries(maxNetworkRetries int64) {
	s.Called(maxNetworkRetries)
}

func SetStripeFixtureResponse(t *testing.T, v stripe.LastResponseSetter, data []byte) {
	err := json.Unmarshal(data, v)
	if err != nil {
		t.Fatal("Unable to unmarshal json fixture")
	}
	v.SetLastResponse(&stripe.APIResponse{
		RawJSON:    data,
		Status:     "200 OK",
		StatusCode: 200,
	})
}
