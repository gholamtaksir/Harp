package scan

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/cockroachdb/errors"
	. "github.com/gholamtaksir/Harp/cmd/shared"
	"github.com/gholamtaksir/Harp/core"
	"github.com/gholamtaksir/Harp/scanner"
	"github.com/gholamtaksir/Harp/warp"
	"github.com/spf13/cobra"
)

var shortMsg = "Finds WARP endpoints that work on this network"

var Cmd = &cobra.Command{
	Use:   "scan",
	Short: shortMsg,
	Long: FormatMessage(shortMsg, `
Sends WireGuard handshakes from your account to random WARP addresses and
ports, measures the ones that answer, then checks that traffic really gets
through the best of them. Use a result with --endpoint, e.g.
  harp proxy --endpoint 188.114.98.2:1701
or let Harp pick one each time with --endpoint auto.`),
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			log.Fatalf("%+v\n", err)
		}
	},
}

var (
	count       int
	ipv6        bool
	verify      int
	top         int
	timeout     time.Duration
	rate        int
	concurrency int
	noReserved  bool
)

func init() {
	flags := Cmd.Flags()
	flags.IntVarP(&count, "count", "n", 600, "Number of endpoints to try")
	flags.BoolVar(&ipv6, "ipv6", false, "Also try IPv6 endpoints")
	flags.IntVar(&verify, "verify", 5, "Look for this many endpoints that carry traffic (0 skips the check)")
	flags.IntVar(&top, "top", 20, "Number of results to print")
	flags.DurationVar(&timeout, "timeout", scanner.DefaultTimeout, "How long to wait for each answer")
	flags.IntVar(&rate, "rate", scanner.DefaultRate, "Handshakes sent per second")
	flags.IntVar(&concurrency, "concurrency", scanner.DefaultConcurrency, "Handshakes in flight")
	flags.BoolVar(&noReserved, "no-reserved", false, "Do not send the WARP client id in the reserved bytes")
}

type row struct {
	result scanner.Result
	trace  *core.Trace
}

func run() error {
	if err := EnsureConfigValidAccount(); err != nil {
		return errors.WithStack(err)
	}
	profile, err := warp.Load(CreateContext(), false)
	if err != nil {
		return errors.WithStack(err)
	}
	reserved := profile.Reserved
	if noReserved {
		reserved = nil
	}
	keys, err := scanner.NewKeys(profile.PrivateKey, profile.PeerPublicKey, reserved)
	if err != nil {
		return errors.WithStack(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	prefixes := append([]netip.Prefix{}, scanner.IPv4Prefixes...)
	if ipv6 {
		prefixes = append(prefixes, scanner.IPv6Prefixes...)
	}
	candidates := scanner.Candidates(count, prefixes, scanner.Ports, KnownEndpoints(profile))
	log.Printf("Trying %d endpoints, press Ctrl+C to stop early", len(candidates))

	var mu sync.Mutex
	last := time.Time{}
	results := keys.Scan(ctx, scanner.Options{
		Candidates:  candidates,
		Timeout:     timeout,
		Rate:        rate,
		Concurrency: concurrency,
	}, func(progress scanner.Progress, _ *scanner.Result) {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(last) < 250*time.Millisecond && progress.Done != progress.Total {
			return
		}
		last = time.Now()
		fmt.Fprintf(os.Stderr, "\r%-9s %d/%d, %d answered   ", progress.Phase, progress.Done, progress.Total, progress.Found)
	})
	fmt.Fprintln(os.Stderr)
	if len(results) == 0 {
		return errors.New("no endpoint answered: this network probably blocks WireGuard")
	}

	rows := make([]row, len(results))
	for i, result := range results {
		rows[i] = row{result: result}
	}
	if verify > 0 && ctx.Err() == nil {
		checkTraffic(ctx, profile, rows)
	}
	printRows(rows, len(candidates))
	return nil
}

// checkTraffic tests the best endpoints until enough of them carry traffic.
func checkTraffic(ctx context.Context, profile *warp.Profile, rows []row) {
	settings := core.DefaultSettings().Warp
	settings.Reserved = !noReserved
	limit := min(len(rows), verify*3)
	log.Printf("Checking that traffic gets through the best endpoints")
	working := 0
	for i := 0; i < limit && working < verify && ctx.Err() == nil; i++ {
		if rows[i].result.Loss() >= 0.5 {
			continue
		}
		trace := core.CheckTraffic(ctx, profile, settings, rows[i].result.Endpoint.String())
		rows[i].trace = &trace
		if trace.OK {
			working++
		}
		fmt.Fprintf(os.Stderr, "\rchecked %d, %d working   ", i+1, working)
	}
	fmt.Fprintln(os.Stderr)
	// Working endpoints first, keeping the latency order otherwise.
	ordered := make([]row, 0, len(rows))
	for pass := 0; pass < 3; pass++ {
		for _, r := range rows {
			rank := 1
			if r.trace != nil {
				rank = map[bool]int{true: 0, false: 2}[r.trace.OK]
			}
			if rank == pass {
				ordered = append(ordered, r)
			}
		}
	}
	copy(rows, ordered)
}

func printRows(rows []row, tried int) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "ENDPOINT\tPING\tJITTER\tLOSS\tTRAFFIC\tDATA CENTER")
	for i, r := range rows {
		if i >= top {
			break
		}
		traffic, location := "not checked", ""
		if r.trace != nil {
			if r.trace.OK {
				traffic = fmt.Sprintf("ok, %d ms", r.trace.LatencyMs)
				city, country := core.ColoLocation(r.trace.Colo)
				location = r.trace.Colo
				if city != "" {
					location += " " + city + ", " + country
				}
			} else {
				traffic = "blocked"
			}
		}
		fmt.Fprintf(writer, "%s\t%d ms\t%d ms\t%.0f%%\t%s\t%s\n", r.result.Endpoint,
			r.result.RTT.Milliseconds(), r.result.Jitter.Milliseconds(), r.result.Loss()*100, traffic, location)
	}
	_ = writer.Flush()
	log.Printf("%d of %d endpoints answered", len(rows), tried)
}
