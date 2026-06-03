package okx

import (
	"errors"
	"net/http"
	"testing"
)

func TestOkxRetryable(t *testing.T) {
	timeoutErr := errors.New("OKX API error: code=50004, msg=API endpoint request timeout.")
	rateErr := errors.New("OKX API error: code=50011, msg=Rate limit reached")
	netErr := errors.New("request failed: context deadline exceeded")
	orderErr := errors.New("OKX API error: code=50004, msg=API endpoint request timeout.")

	if !okxRetryable(http.MethodGet, timeoutErr) {
		t.Fatal("GET balance timeout should be retryable")
	}
	if !okxRetryable(http.MethodGet, rateErr) {
		t.Fatal("GET rate limit should be retryable")
	}
	if !okxRetryable(http.MethodPost, netErr) {
		t.Fatal("POST transport failure should be retryable")
	}
	if okxRetryable(http.MethodPost, orderErr) {
		t.Fatal("POST OKX 50004 must not auto-retry")
	}
}
