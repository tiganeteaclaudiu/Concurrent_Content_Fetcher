package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	SimpleContentRequest      = httptest.NewRequest("GET", "/?offset=0&count=5", nil)
	OffsetContentRequest      = httptest.NewRequest("GET", "/?offset=5&count=5", nil)
	PerformanceRequest        = httptest.NewRequest("GET", "/?offset=0&count=1000000", nil)
	ZeroCountRequest          = httptest.NewRequest("GET", "/?offset=0&count=0", nil)
	ZeroCountSetOffsetRequest = httptest.NewRequest("GET", "/?offset=10&count=0", nil)
	NegativeParametersRequest = httptest.NewRequest("GET", "/?offset=-5&count=-10", nil)
	SetCountRequest           = func(length int) *http.Request {
		return httptest.NewRequest("GET", fmt.Sprintf("/?offset=0&count=%d", length), nil)
	}
)

func runRequest(t *testing.T, srv http.Handler, r *http.Request) (content []*ContentItem) {
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, r)

	if response.Code != 200 {
		t.Fatalf("Response code is %d, want 200", response.Code)
		return
	}

	decoder := json.NewDecoder(response.Body)
	err := decoder.Decode(&content)
	if err != nil {
		t.Fatalf("couldn't decode Response json: %v", err)
	}

	return content
}

func TestResponseCount(t *testing.T) {
	content := runRequest(t, app, SimpleContentRequest)

	assert.Len(t, content, 5)
}

func TestResponseOrder(t *testing.T) {
	content := runRequest(t, app, SimpleContentRequest)

	assert.Len(t, content, 5)

	for i, item := range content {
		assert.Equal(t, Provider(item.Source), DefaultConfig[i%len(DefaultConfig)].Type)
	}
}

func TestOffsetResponseOrder(t *testing.T) {
	content := runRequest(t, app, OffsetContentRequest)

	assert.Len(t, content, 5)

	for i, item := range content {
		// add offset of 5 to i
		assert.Equal(t, Provider(item.Source), DefaultConfig[i+5%len(DefaultConfig)].Type)
	}
}

// TestPerformanceStress : performance stress, meant to attempt to crash main goroutine by stressing it
// Can also be used to assess performance
func TestPerformanceStress(t *testing.T) {
	content := runRequest(t, app, PerformanceRequest)
	assert.Len(t, content, 1000000)
}

// TestZeroCount : tests edge case of count equal to 0 and offset = 0
func TestZeroCount(t *testing.T) {
	content := runRequest(t, app, ZeroCountRequest)

	assert.Len(t, content, 0)
}

// TestNegativeParameters : tests edge case of both count and offset < 0 (should return empty array)
func TestNegativeParameters(t *testing.T) {
	content := runRequest(t, app, NegativeParametersRequest)

	assert.Len(t, content, 0)

}

// TestZeroCountSetOffset : tests edge case of count = 0 and offset > 1 (should return empty array)
func TestZeroCountSetOffset(t *testing.T) {
	content := runRequest(t, app, ZeroCountSetOffsetRequest)

	assert.Len(t, content, 0)

}

// TestCountEqualToProviders : tests edge case of count being equal to length of providers in default config
// Issues might come up because content is fetched in batches equal to the length of providers
func TestCountEqualToProviders(t *testing.T) {
	content := runRequest(t, app, SetCountRequest(len(DefaultConfig)))

	assert.Len(t, content, len(DefaultConfig))

}

// TestCountEqualToProviders : tests edge case of count being equal to length of providers in default config + - 1
// Issues might come up because content is fetched in batches equal to the length of providers
// Adding 1 to count should trigger fetch in another batch, while subtracting one means the first batch should not end
func TestCountEqualToProvidersPlusMinusOne(t *testing.T) {
	content := runRequest(t, app, SetCountRequest(len(DefaultConfig)+1))
	assert.Len(t, content, len(DefaultConfig)+1)

	content = runRequest(t, app, SetCountRequest(len(DefaultConfig)-1))
	assert.Len(t, content, len(DefaultConfig)-1)
}
