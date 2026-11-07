package scanner

import (
	"cmp"
	"context"
	"net/netip"
	"slices"
	"sync"
	"time"
)

// Options control a scan. Zero values select the defaults.
type Options struct {
	// Candidates are the endpoints to probe, see Candidates.
	Candidates []netip.AddrPort
	// Concurrency is the number of probes in flight.
	Concurrency int
	// Rate limits how many probes start per second.
	Rate int
	// Timeout is how long to wait for each answer.
	Timeout time.Duration
	// Samples is the number of handshakes sent to every endpoint that
	// answered the first one, used for latency, jitter and loss.
	Samples int
	// Interval separates the samples sent to one endpoint.
	Interval time.Duration
	// Measure limits the sampling to the fastest endpoints; the others are
	// returned with their single first answer.
	Measure int
}

const (
	DefaultConcurrency = 64
	DefaultRate        = 300
	DefaultTimeout     = 1500 * time.Millisecond
	DefaultSamples     = 5
	DefaultInterval    = 200 * time.Millisecond
	DefaultMeasure     = 40
)

func (o *Options) defaults() {
	if o.Concurrency <= 0 {
		o.Concurrency = DefaultConcurrency
	}
	if o.Rate <= 0 {
		o.Rate = DefaultRate
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.Samples <= 0 {
		o.Samples = DefaultSamples
	}
	if o.Interval <= 0 {
		o.Interval = DefaultInterval
	}
	if o.Measure <= 0 {
		o.Measure = DefaultMeasure
	}
}

// Result describes an endpoint that answered.
type Result struct {
	Endpoint netip.AddrPort
	// RTT is the average handshake round trip time.
	RTT    time.Duration
	MinRTT time.Duration
	// Jitter is the mean difference between consecutive round trips.
	Jitter   time.Duration
	Sent     int
	Received int
}

// Loss is the fraction of handshakes that got no answer.
func (r Result) Loss() float64 {
	if r.Sent == 0 {
		return 1
	}
	return float64(r.Sent-r.Received) / float64(r.Sent)
}

// Progress reports how far a scan got.
type Progress struct {
	// Phase is "probing" while candidates are tried, then "measuring".
	Phase string
	Done  int
	Total int
	Found int
}

// Phases of a scan.
const (
	PhaseProbing   = "probing"
	PhaseMeasuring = "measuring"
)

// Scan probes the candidates and measures every endpoint that answers.
// onProgress, if set, is called from several goroutines; it receives the
// endpoint that just answered, if any. Results are sorted best first.
func (k *Keys) Scan(ctx context.Context, options Options, onProgress func(Progress, *Result)) []Result {
	options.defaults()
	total := len(options.Candidates)
	var (
		mu        sync.Mutex
		done      int
		responses = map[netip.AddrPort][]time.Duration{}
		order     []netip.AddrPort
	)
	report := func(progress Progress, result *Result) {
		if onProgress != nil {
			onProgress(progress, result)
		}
	}

	// Phase 1: one handshake per candidate.
	semaphore := make(chan struct{}, options.Concurrency)
	ticker := time.NewTicker(time.Second / time.Duration(options.Rate))
	var wg sync.WaitGroup
launch:
	for _, candidate := range options.Candidates {
		select {
		case <-ctx.Done():
			break launch
		case <-ticker.C:
		}
		select {
		case <-ctx.Done():
			break launch
		case semaphore <- struct{}{}:
		}
		wg.Add(1)
		go func(candidate netip.AddrPort) {
			defer wg.Done()
			defer func() { <-semaphore }()
			rtt, err := k.Probe(ctx, candidate, options.Timeout)
			mu.Lock()
			done++
			var found *Result
			if err == nil {
				responses[candidate] = []time.Duration{rtt}
				order = append(order, candidate)
				found = &Result{Endpoint: candidate, RTT: rtt, MinRTT: rtt, Sent: 1, Received: 1}
			}
			progress := Progress{Phase: PhaseProbing, Done: done, Total: total, Found: len(order)}
			mu.Unlock()
			report(progress, found)
		}(candidate)
	}
	wg.Wait()
	ticker.Stop()

	// Phase 2: more handshakes to the fastest endpoints that answered.
	slices.SortStableFunc(order, func(a, b netip.AddrPort) int {
		return cmp.Compare(responses[a][0], responses[b][0])
	})
	results := make([]Result, len(order))
	measure := min(len(order), options.Measure)
	measured := 0
	report(Progress{Phase: PhaseMeasuring, Done: 0, Total: measure, Found: len(order)}, nil)
	for i, endpoint := range order {
		if ctx.Err() != nil || i >= measure {
			results[i] = summarize(endpoint, responses[endpoint], 1)
			continue
		}
		semaphore <- struct{}{}
		wg.Add(1)
		go func(i int, endpoint netip.AddrPort) {
			defer wg.Done()
			defer func() { <-semaphore }()
			samples := append([]time.Duration{}, responses[endpoint]...)
			sent := 1
			for sent < options.Samples && ctx.Err() == nil {
				select {
				case <-ctx.Done():
				case <-time.After(options.Interval):
				}
				if ctx.Err() != nil {
					break
				}
				sent++
				if rtt, err := k.Probe(ctx, endpoint, options.Timeout); err == nil {
					samples = append(samples, rtt)
				}
			}
			result := summarize(endpoint, samples, sent)
			mu.Lock()
			results[i] = result
			measured++
			progress := Progress{Phase: PhaseMeasuring, Done: measured, Total: measure, Found: len(order)}
			mu.Unlock()
			report(progress, &result)
		}(i, endpoint)
	}
	wg.Wait()
	Sort(results)
	return results
}

func summarize(endpoint netip.AddrPort, samples []time.Duration, sent int) Result {
	result := Result{Endpoint: endpoint, Sent: sent, Received: len(samples)}
	if len(samples) == 0 {
		return result
	}
	var sum, jitter time.Duration
	result.MinRTT = samples[0]
	for i, sample := range samples {
		sum += sample
		result.MinRTT = min(result.MinRTT, sample)
		if i > 0 {
			jitter += (sample - samples[i-1]).Abs()
		}
	}
	result.RTT = sum / time.Duration(len(samples))
	if len(samples) > 1 {
		result.Jitter = jitter / time.Duration(len(samples)-1)
	}
	return result
}

// Sort orders results by loss, then by latency plus jitter.
func Sort(results []Result) {
	slices.SortStableFunc(results, func(a, b Result) int {
		if lossA, lossB := a.Loss(), b.Loss(); lossA != lossB {
			if lossA < lossB {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.RTT+a.Jitter, b.RTT+b.Jitter)
	})
}

// Quick probes candidates once each and returns the ones that answered,
// fastest first. It is meant for a handful of known endpoints.
func (k *Keys) Quick(ctx context.Context, candidates []netip.AddrPort, timeout time.Duration) []Result {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	results := make([]Result, len(candidates))
	var wg sync.WaitGroup
	for i, candidate := range candidates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := Result{Endpoint: candidate, Sent: 1}
			if rtt, err := k.Probe(ctx, candidate, timeout); err == nil {
				result.RTT, result.MinRTT, result.Received = rtt, rtt, 1
			}
			results[i] = result
		}()
	}
	wg.Wait()
	answered := slices.DeleteFunc(results, func(r Result) bool { return r.Received == 0 })
	slices.SortStableFunc(answered, func(a, b Result) int { return cmp.Compare(a.RTT, b.RTT) })
	return answered
}
