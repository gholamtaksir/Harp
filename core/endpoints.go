package core

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gholamtaksir/Harp/engine"
	"github.com/gholamtaksir/Harp/scanner"
	"github.com/gholamtaksir/Harp/warp"
)

// EventScan carries ScanState updates.
const EventScan = "harp:scan"

// Endpoint statuses, from best to worst.
const (
	// EndpointWorking: traffic got through WARP.
	EndpointWorking = "working"
	// EndpointAnswered: the WireGuard handshake was answered; traffic was
	// not tested.
	EndpointAnswered = "answered"
	// EndpointBlocked: the handshake was answered but no traffic got
	// through, which is how WireGuard filtering usually looks.
	EndpointBlocked = "blocked"
	// EndpointUnreachable: no answer at all.
	EndpointUnreachable = "unreachable"
)

// Scan phases.
const (
	ScanProbing   = scanner.PhaseProbing
	ScanMeasuring = scanner.PhaseMeasuring
	ScanVerifying = "verifying"
	ScanDone      = "done"
	ScanCancelled = "cancelled"
	ScanFailed    = "failed"
)

// Scan sizes: how many endpoints are tried.
var scanSizes = map[string]int{"quick": 300, "standard": 1000, "thorough": 3000}

const (
	defaultVerify     = 5
	verifyTimeout     = 7 * time.Second
	verifyWorkers     = 3
	failedEndpointTTL = 15 * time.Minute
	maxStoredResults  = 100
	scanEmitInterval  = 150 * time.Millisecond
	knownProbeTimeout = 2 * time.Second
	knownProbeLimit   = 16
)

// ErrNoEndpoint is returned when no WARP endpoint answers.
var ErrNoEndpoint = errors.New("no WARP endpoint answers on this network; it may block WireGuard entirely")

// EndpointResult is what is known about one WARP endpoint.
type EndpointResult struct {
	Endpoint string `json:"endpoint"`
	// Ping is the average WireGuard handshake round trip in milliseconds.
	Ping    float64 `json:"ping"`
	MinPing float64 `json:"minPing"`
	Jitter  float64 `json:"jitter"`
	// Loss is the percentage of unanswered handshakes.
	Loss   float64 `json:"loss"`
	Status string  `json:"status"`
	// Colo is the Cloudflare data center traffic came out of, with its
	// city and country when known.
	Colo    string `json:"colo"`
	City    string `json:"city"`
	Country string `json:"country"`
	// Latency is the duration of an HTTPS request through the tunnel.
	Latency   int64  `json:"latency"`
	Error     string `json:"error"`
	CheckedAt int64  `json:"checkedAt"`
}

// ScanRequest configures a scan started by the user.
type ScanRequest struct {
	// Size is quick, standard or thorough.
	Size string `json:"size"`
	IPv6 bool   `json:"ipv6"`
	// Verify is how many working endpoints to look for by sending traffic.
	Verify int `json:"verify"`
}

// ScanState describes the current or last scan.
type ScanState struct {
	Running bool   `json:"running"`
	Phase   string `json:"phase"`
	// Done and Total are the progress of the current phase.
	Done  int `json:"done"`
	Total int `json:"total"`
	// Probed and Candidates count the endpoints tried, Found those that
	// answered.
	Probed     int    `json:"probed"`
	Candidates int    `json:"candidates"`
	Found      int    `json:"found"`
	Size       string `json:"size"`
	Automatic  bool   `json:"automatic"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
	Error      string `json:"error"`
	// Last is the endpoint that most recently carried traffic.
	Last    string           `json:"last"`
	Results []EndpointResult `json:"results"`
}

// endpointDB is saved to Paths.Endpoints.
type endpointDB struct {
	Last       string           `json:"last"`
	Results    []EndpointResult `json:"results"`
	ScannedAt  int64            `json:"scannedAt"`
	Size       string           `json:"size"`
	Candidates int              `json:"candidates"`
	Found      int              `json:"found"`
}

// endpoints owns scanning and the endpoint database.
type endpoints struct {
	path string
	// Where scans look; replaced in tests.
	wellKnown  []netip.AddrPort
	prefixesV4 []netip.Prefix
	prefixesV6 []netip.Prefix

	mu       sync.Mutex
	db       endpointDB
	state    ScanState
	cancel   context.CancelFunc
	failed   map[string]time.Time
	lastEmit time.Time
}

func newEndpoints(path string) *endpoints {
	e := &endpoints{
		path:       path,
		failed:     map[string]time.Time{},
		wellKnown:  scanner.WellKnown,
		prefixesV4: scanner.IPv4Prefixes,
		prefixesV6: scanner.IPv6Prefixes,
	}
	if path != "" {
		if content, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(content, &e.db)
		}
	}
	if e.db.Results == nil {
		e.db.Results = []EndpointResult{}
	}
	e.state = ScanState{Results: e.db.Results, Last: e.db.Last, Size: e.db.Size,
		Candidates: e.db.Candidates, Probed: e.db.Candidates, Found: e.db.Found, FinishedAt: e.db.ScannedAt}
	if e.db.ScannedAt != 0 {
		e.state.Phase = ScanDone
	}
	return e
}

// snapshot returns a copy of the scan state. Callers hold e.mu.
func (e *endpoints) snapshotLocked() ScanState {
	state := e.state
	state.Last = e.db.Last
	state.Results = slices.Clone(state.Results)
	if state.Results == nil {
		state.Results = []EndpointResult{}
	}
	return state
}

// saveLocked writes the database. Callers hold e.mu.
func (e *endpoints) saveLocked() error {
	if e.path == "" {
		return nil
	}
	return writeJSONAtomic(e.path, e.db)
}

func writeJSONAtomic(path string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return errors.WithStack(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errors.WithStack(err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, content, 0o600); err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(os.Rename(temporary, path))
}

func statusRank(status string) int {
	switch status {
	case EndpointWorking:
		return 0
	case EndpointAnswered:
		return 1
	case EndpointBlocked:
		return 2
	}
	return 3
}

// sortResults puts working endpoints first, then by loss and ping.
func sortResults(results []EndpointResult) {
	slices.SortStableFunc(results, func(a, b EndpointResult) int {
		if c := cmp.Compare(statusRank(a.Status), statusRank(b.Status)); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Loss, b.Loss); c != 0 {
			return c
		}
		return cmp.Compare(a.Ping+a.Jitter, b.Ping+b.Jitter)
	})
}

func milliseconds(d time.Duration) float64 {
	return math.Round(float64(d)/float64(time.Millisecond)*10) / 10
}

func fromScan(result scanner.Result) EndpointResult {
	return EndpointResult{
		Endpoint:  result.Endpoint.String(),
		Ping:      milliseconds(result.RTT),
		MinPing:   milliseconds(result.MinRTT),
		Jitter:    milliseconds(result.Jitter),
		Loss:      math.Round(result.Loss() * 100),
		Status:    EndpointAnswered,
		CheckedAt: time.Now().UnixMilli(),
	}
}

// applyTrace records the outcome of sending traffic through an endpoint.
func applyTrace(result *EndpointResult, trace Trace) {
	result.CheckedAt = time.Now().UnixMilli()
	if trace.OK {
		result.Status = EndpointWorking
		result.Colo = trace.Colo
		result.City, result.Country = ColoLocation(trace.Colo)
		if result.Country == "" {
			result.Country = trace.Location
		}
		result.Latency = trace.LatencyMs
		result.Error = ""
		return
	}
	result.Status = EndpointBlocked
	result.Error = trace.Error
}

// upsertLocked updates or adds a result in the database and the scan state.
func (e *endpoints) upsertLocked(update func(*EndpointResult), endpoint string) {
	apply := func(results []EndpointResult) []EndpointResult {
		for i := range results {
			if results[i].Endpoint == endpoint {
				update(&results[i])
				sortResults(results)
				return results
			}
		}
		result := EndpointResult{Endpoint: endpoint}
		update(&result)
		results = append(results, result)
		sortResults(results)
		return results
	}
	e.db.Results = apply(slices.Clone(e.db.Results))
	if len(e.db.Results) > maxStoredResults {
		e.db.Results = e.db.Results[:maxStoredResults]
	}
	if !e.state.Running {
		e.state.Results = slices.Clone(e.db.Results)
	} else {
		e.state.Results = apply(e.state.Results)
	}
}

// ---- Controller integration ----

func scanKeys(profile *warp.Profile, settings WarpSettings) (*scanner.Keys, error) {
	reserved := profile.Reserved
	if !settings.Reserved {
		reserved = nil
	}
	return scanner.NewKeys(profile.PrivateKey, profile.PeerPublicKey, reserved)
}

// ScanState returns the current or last scan.
func (c *Controller) ScanState() ScanState {
	c.endpoints.mu.Lock()
	defer c.endpoints.mu.Unlock()
	return c.endpoints.snapshotLocked()
}

// emitScan sends the scan state; unless force is set, at most every
// scanEmitInterval.
func (c *Controller) emitScan(force bool) {
	e := c.endpoints
	e.mu.Lock()
	if !force && time.Since(e.lastEmit) < scanEmitInterval {
		e.mu.Unlock()
		return
	}
	e.lastEmit = time.Now()
	state := e.snapshotLocked()
	e.mu.Unlock()
	c.emit(EventScan, state)
}

// StartScan looks for WARP endpoints that answer on this network. A running
// connection is stopped first, since scanning through the tunnel would test
// the wrong path.
func (c *Controller) StartScan(request ScanRequest) error {
	if status := c.State().Status; status == StatusConnected || status == StatusConnecting {
		if err := c.Disconnect(); err != nil {
			return err
		}
	}
	_, err := c.startScan(context.Background(), request, false)
	return err
}

// StopScan cancels a running scan, keeping what was found.
func (c *Controller) StopScan() {
	c.endpoints.mu.Lock()
	cancel := c.endpoints.cancel
	c.endpoints.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ClearEndpoints forgets all scan results.
func (c *Controller) ClearEndpoints() error {
	e := c.endpoints
	e.mu.Lock()
	if e.state.Running {
		e.mu.Unlock()
		return errors.New("stop the scan first")
	}
	e.db = endpointDB{Results: []EndpointResult{}}
	e.state = ScanState{Results: []EndpointResult{}}
	e.failed = map[string]time.Time{}
	err := e.saveLocked()
	e.mu.Unlock()
	c.emitScan(true)
	return err
}

func (c *Controller) startScan(parent context.Context, request ScanRequest, automatic bool) (<-chan struct{}, error) {
	if !c.accounts.Exists() {
		return nil, ErrNoAccount
	}
	profile, err := c.accounts.Profile(false)
	if err != nil {
		return nil, err
	}
	settings := c.Settings()
	keys, err := scanKeys(profile, settings.Warp)
	if err != nil {
		return nil, err
	}
	if _, ok := scanSizes[request.Size]; !ok {
		request.Size = "quick"
	}
	if request.Verify < 0 {
		request.Verify = 0
	}

	e := c.endpoints
	e.mu.Lock()
	if e.state.Running {
		e.mu.Unlock()
		return nil, errors.New("a scan is already running")
	}
	ctx, cancel := context.WithCancel(parent)
	e.cancel = cancel
	e.state = ScanState{
		Running:   true,
		Phase:     ScanProbing,
		Size:      request.Size,
		Automatic: automatic,
		StartedAt: time.Now().UnixMilli(),
		Results:   []EndpointResult{},
	}
	e.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel()
		c.runScan(ctx, keys, profile, settings, request, automatic)
	}()
	return done, nil
}

func (c *Controller) runScan(ctx context.Context, keys *scanner.Keys, profile *warp.Profile, settings Settings, request ScanRequest, automatic bool) {
	e := c.endpoints
	prefixes := slices.Clone(e.prefixesV4)
	if request.IPv6 {
		prefixes = append(prefixes, e.prefixesV6...)
	}
	candidates := scanner.Candidates(scanSizes[request.Size], prefixes, scanner.Ports, c.knownCandidates(profile, false))
	measure := scanner.DefaultMeasure
	if automatic {
		// Connecting waits for this scan, so only the best are sampled.
		measure = 10
	}
	c.logs.add("info", fmt.Sprintf("Scanning %d WARP endpoints", len(candidates)))

	e.mu.Lock()
	e.state.Total = len(candidates)
	e.state.Candidates = len(candidates)
	e.mu.Unlock()
	c.emitScan(true)

	results := keys.Scan(ctx, scanner.Options{Candidates: candidates, Measure: measure}, func(progress scanner.Progress, result *scanner.Result) {
		e.mu.Lock()
		if progress.Phase != e.state.Phase && progress.Phase == scanner.PhaseMeasuring {
			e.state.Probed = e.state.Done
		}
		e.state.Phase = progress.Phase
		e.state.Done, e.state.Total, e.state.Found = progress.Done, progress.Total, progress.Found
		if progress.Phase == scanner.PhaseProbing {
			e.state.Probed = progress.Done
		}
		if result != nil {
			converted := fromScan(*result)
			e.state.Results = upsertResult(e.state.Results, converted)
		}
		e.mu.Unlock()
		c.emitScan(false)
	})

	converted := make([]EndpointResult, len(results))
	for i, result := range results {
		converted[i] = fromScan(result)
	}
	e.mu.Lock()
	e.state.Results = converted
	e.state.Found = len(converted)
	e.mu.Unlock()

	if request.Verify > 0 && ctx.Err() == nil && len(converted) != 0 {
		c.verifyResults(ctx, profile, settings, request.Verify)
	}

	e.mu.Lock()
	sortResults(e.state.Results)
	e.state.Running = false
	e.state.FinishedAt = time.Now().UnixMilli()
	e.state.Phase = ScanDone
	if ctx.Err() != nil {
		e.state.Phase = ScanCancelled
	}
	// A scan replaces older results, except for working endpoints it did
	// not get to test.
	merged := slices.Clone(e.state.Results)
	for _, old := range e.db.Results {
		if old.Status == EndpointWorking && !slices.ContainsFunc(merged, func(r EndpointResult) bool { return r.Endpoint == old.Endpoint }) {
			merged = append(merged, old)
		}
	}
	sortResults(merged)
	if len(merged) > maxStoredResults {
		merged = merged[:maxStoredResults]
	}
	e.db.Results = merged
	e.db.ScannedAt = e.state.FinishedAt
	e.db.Size = e.state.Size
	e.db.Candidates = e.state.Candidates
	e.db.Found = e.state.Found
	e.state.Results = slices.Clone(merged)
	if err := e.saveLocked(); err != nil {
		c.logs.add("warn", "Could not save the endpoint list: "+err.Error())
	}
	found, probed := e.state.Found, e.state.Probed
	working := 0
	for _, result := range merged {
		if result.Status == EndpointWorking {
			working++
		}
	}
	e.cancel = nil
	e.mu.Unlock()
	c.emitScan(true)

	message := fmt.Sprintf("Scan finished: %d of %d endpoints answered", found, probed)
	if request.Verify > 0 {
		message += fmt.Sprintf(", %d carry traffic", working)
	}
	c.logs.add("info", message)
}

func upsertResult(results []EndpointResult, result EndpointResult) []EndpointResult {
	for i := range results {
		if results[i].Endpoint == result.Endpoint {
			results[i] = result
			return results
		}
	}
	return append(results, result)
}

// verifyResults sends traffic through the best endpoints of the running scan
// until want of them work.
func (c *Controller) verifyResults(ctx context.Context, profile *warp.Profile, settings Settings, want int) {
	e := c.endpoints
	e.mu.Lock()
	var queue []string
	for _, result := range e.state.Results {
		if result.Loss < 50 {
			queue = append(queue, result.Endpoint)
		}
	}
	maximum := min(len(queue), want*3)
	queue = queue[:maximum]
	e.state.Phase = ScanVerifying
	e.state.Done, e.state.Total = 0, maximum
	e.mu.Unlock()
	c.emitScan(true)

	var (
		mu      sync.Mutex
		next    int
		working int
		wg      sync.WaitGroup
	)
	for range verifyWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if next >= len(queue) || working >= want || ctx.Err() != nil {
					mu.Unlock()
					return
				}
				endpoint := queue[next]
				next++
				mu.Unlock()

				trace := CheckTraffic(ctx, profile, settings.Warp, endpoint)
				if ctx.Err() != nil {
					return
				}
				mu.Lock()
				if trace.OK {
					working++
				}
				mu.Unlock()
				e.mu.Lock()
				for i := range e.state.Results {
					if e.state.Results[i].Endpoint == endpoint {
						applyTrace(&e.state.Results[i], trace)
					}
				}
				e.state.Done++
				e.mu.Unlock()
				c.emitScan(true)
			}
		}()
	}
	wg.Wait()
}

// CheckTraffic starts a tunnel to endpoint and fetches the trace through it,
// telling whether traffic gets through.
func CheckTraffic(ctx context.Context, profile *warp.Profile, settings WarpSettings, endpoint string) Trace {
	settings.Endpoint = endpoint
	w, err := WarpEndpoint(settings, profile)
	if err != nil {
		return Trace{Error: err.Error(), CheckedAt: time.Now().UnixMilli()}
	}
	instance, err := engine.StartWarp(w)
	if err != nil {
		return Trace{Error: err.Error(), CheckedAt: time.Now().UnixMilli()}
	}
	defer func() { go instance.Close() }()
	client, err := instance.HTTPClient(engine.DefaultWarpTag, verifyTimeout)
	if err != nil {
		return Trace{Error: err.Error(), CheckedAt: time.Now().UnixMilli()}
	}
	return FetchTraceContext(ctx, client)
}

// TestEndpoint measures one endpoint and checks whether traffic gets
// through it, updating the stored results.
func (c *Controller) TestEndpoint(endpoint string) (EndpointResult, error) {
	address, err := netip.ParseAddrPort(endpoint)
	if err != nil {
		return EndpointResult{}, errors.Newf("invalid endpoint %q (expected ip:port)", endpoint)
	}
	endpoint = address.String()
	profile, err := c.accounts.Profile(false)
	if err != nil {
		return EndpointResult{}, err
	}
	settings := c.Settings()
	keys, err := scanKeys(profile, settings.Warp)
	if err != nil {
		return EndpointResult{}, err
	}
	ctx := context.Background()
	measured := keys.Scan(ctx, scanner.Options{Candidates: []netip.AddrPort{address}, Samples: 4}, nil)
	result := EndpointResult{Endpoint: endpoint, Status: EndpointUnreachable, Loss: 100, CheckedAt: time.Now().UnixMilli()}
	if len(measured) != 0 {
		result = fromScan(measured[0])
		applyTrace(&result, CheckTraffic(ctx, profile, settings.Warp, endpoint))
	}
	c.endpoints.mu.Lock()
	c.endpoints.upsertLocked(func(r *EndpointResult) { *r = result }, endpoint)
	if result.Status != EndpointWorking {
		c.endpoints.failed[endpoint] = time.Now()
	} else {
		delete(c.endpoints.failed, endpoint)
	}
	err = c.endpoints.saveLocked()
	c.endpoints.mu.Unlock()
	c.emitScan(true)
	return result, err
}

// knownCandidates lists endpoints in order of preference: the last one that
// worked, other working ones, ones that answered a scan, the account's own
// endpoint and well-known addresses.
func (c *Controller) knownCandidates(profile *warp.Profile, skipFailed bool) []netip.AddrPort {
	e := c.endpoints
	e.mu.Lock()
	db := e.db
	db.Results = slices.Clone(db.Results)
	failed := map[string]bool{}
	for endpoint, at := range e.failed {
		if time.Since(at) < failedEndpointTTL {
			failed[endpoint] = true
		}
	}
	e.mu.Unlock()

	var result []netip.AddrPort
	seen := map[netip.AddrPort]bool{}
	add := func(endpoint string) {
		address, err := netip.ParseAddrPort(endpoint)
		if err != nil || seen[address] || (skipFailed && failed[address.String()]) {
			return
		}
		seen[address] = true
		result = append(result, address)
	}
	add(db.Last)
	for _, status := range []string{EndpointWorking, EndpointAnswered} {
		count := 0
		for _, r := range db.Results {
			if r.Status == status && r.Loss < 50 && count < 10 {
				add(r.Endpoint)
				count++
			}
		}
	}
	if profile != nil {
		if server, port, err := engine.ResolveEndpoint(profile.Endpoint, profile.EndpointIP); err == nil {
			add(net.JoinHostPort(server, strconv.Itoa(int(port))))
		}
	}
	for _, address := range e.wellKnown {
		add(address.String())
	}
	return result
}

// chooseEndpoint picks the endpoint for automatic mode: the most preferred
// known endpoint that answers, or the best one a quick scan finds.
func (c *Controller) chooseEndpoint(ctx context.Context, profile *warp.Profile, settings Settings) (string, error) {
	keys, err := scanKeys(profile, settings.Warp)
	if err != nil {
		return "", err
	}
	candidates := c.knownCandidates(profile, true)
	if len(candidates) > knownProbeLimit {
		candidates = candidates[:knownProbeLimit]
	}
	answered := keys.Quick(ctx, candidates, knownProbeTimeout)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if len(answered) != 0 {
		ok := map[netip.AddrPort]scanner.Result{}
		for _, result := range answered {
			ok[result.Endpoint] = result
		}
		for _, candidate := range candidates {
			if result, found := ok[candidate]; found {
				c.logs.add("info", fmt.Sprintf("Using endpoint %s (%d of %d known endpoints answered, %.0f ms)",
					candidate, len(answered), len(candidates), milliseconds(result.RTT)))
				return candidate.String(), nil
			}
		}
	}

	c.logs.add("warn", "None of the known WARP endpoints answered; scanning for one")
	done, err := c.startScan(ctx, ScanRequest{Size: "quick"}, true)
	if err != nil {
		return "", err
	}
	select {
	case <-done:
	case <-ctx.Done():
		<-done
		return "", ctx.Err()
	}
	e := c.endpoints
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, result := range e.db.Results {
		if (result.Status == EndpointAnswered || result.Status == EndpointWorking) &&
			!(time.Since(e.failed[result.Endpoint]) < failedEndpointTTL) {
			return result.Endpoint, nil
		}
	}
	return "", ErrNoEndpoint
}

// markEndpoint records whether traffic got through endpoint.
func (c *Controller) markEndpoint(endpoint string, trace Trace) {
	if _, err := netip.ParseAddrPort(endpoint); err != nil {
		return
	}
	e := c.endpoints
	e.mu.Lock()
	e.upsertLocked(func(result *EndpointResult) { applyTrace(result, trace) }, endpoint)
	if trace.OK {
		e.db.Last = endpoint
		delete(e.failed, endpoint)
	} else {
		e.failed[endpoint] = time.Now()
		if e.db.Last == endpoint {
			e.db.Last = ""
		}
	}
	err := e.saveLocked()
	e.mu.Unlock()
	if err != nil {
		c.logs.add("warn", "Could not save the endpoint list: "+err.Error())
	}
	c.emitScan(true)
}

// lastEndpoint is the endpoint that most recently carried traffic.
func (c *Controller) lastEndpoint() string {
	c.endpoints.mu.Lock()
	defer c.endpoints.mu.Unlock()
	return c.endpoints.db.Last
}

// NextEndpoint moves a running connection to another endpoint. The current
// endpoint is skipped for a while. In automatic mode Harp then picks the next
// best one; with a fixed endpoint, the next known endpoint that answers
// becomes the fixed one. It returns the new endpoint.
func (c *Controller) NextEndpoint() (string, error) {
	state := c.State()
	if state.Status != StatusConnected {
		return "", errors.New("not connected")
	}
	current := state.Endpoint
	c.endpoints.mu.Lock()
	c.endpoints.failed[current] = time.Now()
	c.endpoints.mu.Unlock()

	settings := c.Settings()
	if settings.Warp.Endpoint == "" {
		c.operation.Lock()
		defer c.operation.Unlock()
		c.failovers = 0
		c.notice = fmt.Sprintf("Switching from %s to the next endpoint…", current)
		c.logs.add("info", c.notice)
		if err := c.disconnect(); err != nil {
			return "", err
		}
		if err := c.connect(); err != nil {
			return "", err
		}
		return c.State().Endpoint, nil
	}

	profile, err := c.accounts.Profile(false)
	if err != nil {
		return "", err
	}
	keys, err := scanKeys(profile, settings.Warp)
	if err != nil {
		return "", err
	}
	candidates := c.knownCandidates(profile, true)
	if len(candidates) > knownProbeLimit {
		candidates = candidates[:knownProbeLimit]
	}
	answered := map[netip.AddrPort]bool{}
	for _, result := range keys.Quick(context.Background(), candidates, knownProbeTimeout) {
		answered[result.Endpoint] = true
	}
	for _, candidate := range candidates {
		if answered[candidate] && candidate.String() != current {
			c.logs.add("info", fmt.Sprintf("Switching from %s to %s", current, candidate))
			settings.Warp.Endpoint = candidate.String()
			if err := c.SaveSettings(settings); err != nil {
				return "", err
			}
			return candidate.String(), nil
		}
	}
	return "", errors.New("no other known endpoint answers; run a scan on the Endpoints page")
}
