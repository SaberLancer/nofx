package market

import (
	"io"
	"net/http"
	"nofx/security"
	"strconv"
	"time"
)

func okxPublicGet(url string, simulated bool, timeout time.Duration) ([]byte, error) {
	if err := security.ValidateURL(url); err != nil {
		return nil, err
	}

	client := security.SafeHTTPClient(timeout)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if simulated {
		req.Header.Set("x-simulated-trading", "1")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &okxHTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	return body, nil
}

type okxHTTPError struct {
	StatusCode int
	Body       string
}

func (e *okxHTTPError) Error() string {
	return "okx public api returned status " + strconv.Itoa(e.StatusCode) + ": " + e.Body
}
