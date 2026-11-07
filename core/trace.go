package core

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

// traceURL is fetched through WARP. An IP address keeps the check free of
// DNS, so it only depends on the tunnel itself.
const traceURL = "https://1.1.1.1/cdn-cgi/trace"

// Trace is the result of Cloudflare's connection trace, fetched through WARP.
type Trace struct {
	OK        bool   `json:"ok"`
	IP        string `json:"ip"`
	Location  string `json:"location"`
	Colo      string `json:"colo"`
	Warp      string `json:"warp"`
	TLS       string `json:"tls"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error"`
	CheckedAt int64  `json:"checkedAt"`
}

// FetchTrace requests the trace with client.
func FetchTrace(client *http.Client) Trace {
	return FetchTraceContext(context.Background(), client)
}

// FetchTraceContext requests the trace with client until ctx is done.
func FetchTraceContext(ctx context.Context, client *http.Client) Trace {
	started := time.Now()
	result := Trace{CheckedAt: started.UnixMilli()}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, traceURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	response, err := client.Do(request)
	if err != nil {
		result.Error = simplifyNetworkError(err)
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		result.Error = "unexpected response: " + response.Status
		return result
	}
	values, err := parseTrace(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.LatencyMs = time.Since(started).Milliseconds()
	result.IP = values["ip"]
	result.Location = values["loc"]
	result.Colo = values["colo"]
	result.Warp = values["warp"]
	result.TLS = values["tls"]
	result.OK = true
	return result
}

func parseTrace(reader io.Reader) (map[string]string, error) {
	values := map[string]string{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if key, value, found := strings.Cut(scanner.Text(), "="); found {
			values[key] = value
		}
	}
	return values, errors.WithStack(scanner.Err())
}

func simplifyNetworkError(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "Client.Timeout"), strings.Contains(message, "deadline exceeded"):
		return "timed out: WARP did not respond; the endpoint may be blocked on this network"
	case strings.Contains(message, "i/o timeout"):
		return "timed out: no response through WARP"
	}
	return message
}
