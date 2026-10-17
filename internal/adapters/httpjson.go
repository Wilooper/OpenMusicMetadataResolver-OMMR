package adapters

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func ReadJSONBody(resp *http.Response) ([]byte, error) {
	const maxBody = 8 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("provider JSON response exceeds size limit")
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("provider returned a non-JSON response (HTTP %d)", resp.StatusCode)
	}
	return body, nil
}
