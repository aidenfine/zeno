// Command bench is a small load generator for zeno, in the spirit of
// redis-benchmark. It opens N concurrent TCP connections, fires RESP commands
// at the server, and reports throughput and latency percentiles.
//
// Because every client command is forwarded to the leader over gRPC (see
// controlplane.SendCommand), the target address must be a reachable *leader*. In
// practice that is the a1 node from docker-compose.local.yml, which publishes
// 6379/6380 to the host:
//
//	docker compose -f docker-compose.local.yml up --build
//	go run ./bench -addr 127.0.0.1:6379 -cmd SET -clients 50 -n 100000
//
// A single locally-run node (`make run`) is NOT the leader, so it tries to
// forward to 10.10.1.10 and every request comes back as "ERR ..." after the
// 2s dial timeout. The benchmark surfaces that as a high error count and
// ~2s latencies, rather than reporting misleadingly fast throughput.
//
// For network-free, always-runnable component numbers use the Go benchmarks
// instead: `make bench`.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type config struct {
	addr      string
	clients   int
	requests  int
	cmd       string
	keyspace  int
	valueSize int
	pipeline  int
	timeout   time.Duration
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.addr, "addr", "127.0.0.1:6379", "zeno leader address (host:port)")
	flag.IntVar(&cfg.clients, "clients", 50, "number of concurrent connections")
	flag.IntVar(&cfg.requests, "n", 100_000, "total number of requests to send")
	flag.StringVar(&cfg.cmd, "cmd", "SET", "command to benchmark: SET, GET, PING, or MIXED")
	flag.IntVar(&cfg.keyspace, "keyspace", 1000, "number of distinct keys to spread requests over")
	flag.IntVar(&cfg.valueSize, "valuesize", 16, "SET value size in bytes")
	flag.IntVar(&cfg.pipeline, "pipeline", 1, "number of commands to pipeline per round trip")
	flag.DurationVar(&cfg.timeout, "timeout", 10*time.Second, "dial + per-batch I/O timeout")
	flag.Parse()

	cfg.cmd = strings.ToUpper(cfg.cmd)
	return cfg
}

func main() {
	cfg := parseFlags()
	if err := validate(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "invalid config:", err)
		os.Exit(2)
	}

	printHeader(cfg)

	// Fail fast with a clear message if nothing is listening.
	probe, err := net.DialTimeout("tcp", cfg.addr, cfg.timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\ncannot reach %s: %v\nis a zeno leader running? see the docs at the top of bench/main.go\n", cfg.addr, err)
		os.Exit(1)
	}
	probe.Close()

	stats := run(cfg)
	report(cfg, stats)
}

func validate(cfg config) error {
	switch cfg.cmd {
	case "SET", "GET", "PING", "MIXED":
	default:
		return fmt.Errorf("unknown -cmd %q (want SET, GET, PING, or MIXED)", cfg.cmd)
	}
	if cfg.clients < 1 {
		return fmt.Errorf("-clients must be >= 1")
	}
	if cfg.requests < 1 {
		return fmt.Errorf("-n must be >= 1")
	}
	if cfg.pipeline < 1 {
		return fmt.Errorf("-pipeline must be >= 1")
	}
	return nil
}

// stats aggregates the outcome of a run.
type stats struct {
	latencies []time.Duration // one sample per round trip (batch)
	ops       int64           // successful command replies
	appErrs   int64           // replies that came back as "ERR ..."
	ioErrs    int64           // connection/protocol failures
	wall      time.Duration
	errSample atomic.Value // string: first application/IO error seen
}

func run(cfg config) *stats {
	s := &stats{}

	// Split the request budget across clients. Each client keeps its own
	// latency slice so the hot path is lock-free; we merge at the end.
	perClient := splitWork(cfg.requests, cfg.clients)
	latPerClient := make([][]time.Duration, cfg.clients)

	var wg sync.WaitGroup
	start := time.Now()
	for c := range cfg.clients {
		wg.Add(1)
		go func(id, budget int) {
			defer wg.Done()
			latPerClient[id] = runClient(cfg, id, budget, s)
		}(c, perClient[c])
	}
	wg.Wait()
	s.wall = time.Since(start)

	for _, l := range latPerClient {
		s.latencies = append(s.latencies, l...)
	}
	slices.Sort(s.latencies)
	return s
}

// runClient drives one connection through `budget` requests and returns its
// per-round-trip latencies.
func runClient(cfg config, id, budget int, s *stats) []time.Duration {
	conn, err := net.DialTimeout("tcp", cfg.addr, cfg.timeout)
	if err != nil {
		atomic.AddInt64(&s.ioErrs, int64(budget))
		recordErr(s, err.Error())
		return nil
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	// Each client seeds its own RNG deterministically from its id so runs are
	// reproducible while clients still touch different keys.
	rng := rand.New(rand.NewSource(int64(id) + 1))
	value := strings.Repeat("x", cfg.valueSize)

	lat := make([]time.Duration, 0, budget/max(cfg.pipeline, 1)+1)

	for done := 0; done < budget; {
		batch := min(cfg.pipeline, budget-done)

		conn.SetDeadline(time.Now().Add(cfg.timeout))
		t0 := time.Now()

		for i := range batch {
			if err := writeCommand(w, buildArgs(cfg, rng, value)); err != nil {
				atomic.AddInt64(&s.ioErrs, int64(batch-i))
				recordErr(s, err.Error())
				return lat
			}
		}
		if err := w.Flush(); err != nil {
			atomic.AddInt64(&s.ioErrs, int64(batch))
			recordErr(s, err.Error())
			return lat
		}

		failed := false
		for i := range batch {
			reply, err := readReply(r)
			if err != nil {
				atomic.AddInt64(&s.ioErrs, int64(batch-i))
				recordErr(s, err.Error())
				failed = true
				break
			}
			if strings.HasPrefix(reply, "ERR") {
				atomic.AddInt64(&s.appErrs, 1)
				recordErr(s, reply)
			} else {
				atomic.AddInt64(&s.ops, 1)
			}
		}
		if failed {
			return lat
		}

		lat = append(lat, time.Since(t0))
		done += batch
	}
	return lat
}

// buildArgs returns the RESP argument list for one command. value is the
// precomputed SET payload, reused to keep the payload out of the hot path.
func buildArgs(cfg config, rng *rand.Rand, value string) []string {
	key := "key:" + strconv.Itoa(rng.Intn(cfg.keyspace))
	switch cfg.cmd {
	case "PING":
		return []string{"PING"}
	case "GET":
		return []string{"GET", key}
	case "SET":
		return []string{"SET", key, value}
	case "MIXED": // ~1 write per 4 reads, a read-heavy cache profile
		if rng.Intn(5) == 0 {
			return []string{"SET", key, value}
		}
		return []string{"GET", key}
	default:
		return []string{"PING"}
	}
}

// writeCommand encodes args as a RESP array of bulk strings.
func writeCommand(w *bufio.Writer, args []string) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, a := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(a), a); err != nil {
			return err
		}
	}
	return nil
}

// readReply consumes exactly one RESP reply and returns its simple-string /
// error / bulk text so the caller can spot application errors like zeno's
// "ERR ...". It handles every reply type (+, -, :, $, *) — unlike zeno's own
// resp.Read, which only understands arrays and bulk strings.
func readReply(r *bufio.Reader) (string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	switch prefix {
	case '+', '-', ':': // simple string, error, integer
		return readLine(r)
	case '$': // bulk string
		line, err := readLine(r)
		if err != nil {
			return "", err
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			return "", fmt.Errorf("bad bulk length %q: %w", line, err)
		}
		if n < 0 {
			return "", nil // null bulk ($-1)
		}
		buf := make([]byte, n+2) // value + trailing CRLF
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		return string(buf[:n]), nil
	case '*': // array
		line, err := readLine(r)
		if err != nil {
			return "", err
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			return "", fmt.Errorf("bad array length %q: %w", line, err)
		}
		for range max(n, 0) {
			if _, err := readReply(r); err != nil {
				return "", err
			}
		}
		return "", nil
	default:
		return "", fmt.Errorf("unknown reply type %q", string(prefix))
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func recordErr(s *stats, msg string) {
	s.errSample.CompareAndSwap(nil, msg)
}

// splitWork distributes total across n buckets as evenly as possible.
func splitWork(total, n int) []int {
	out := make([]int, n)
	base, rem := total/n, total%n
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
	}
	return out
}

func printHeader(cfg config) {
	fmt.Printf("zeno load test\n")
	fmt.Printf("  target    : %s\n", cfg.addr)
	fmt.Printf("  command   : %s\n", cfg.cmd)
	fmt.Printf("  requests  : %d\n", cfg.requests)
	fmt.Printf("  clients   : %d\n", cfg.clients)
	fmt.Printf("  pipeline  : %d\n", cfg.pipeline)
	fmt.Printf("  keyspace  : %d\n", cfg.keyspace)
	if cfg.cmd == "SET" || cfg.cmd == "MIXED" {
		fmt.Printf("  valuesize : %d bytes\n", cfg.valueSize)
	}
	fmt.Println()
}

func report(cfg config, s *stats) {
	total := s.ops + s.appErrs + s.ioErrs
	fmt.Printf("results\n")
	fmt.Printf("  wall time : %s\n", s.wall.Round(time.Millisecond))
	fmt.Printf("  completed : %d ok, %d app-errors, %d io-errors\n", s.ops, s.appErrs, s.ioErrs)
	if s.wall > 0 {
		fmt.Printf("  throughput: %.0f ops/sec\n", float64(s.ops)/s.wall.Seconds())
	}

	if len(s.latencies) > 0 {
		unit := "per request"
		if cfg.pipeline > 1 {
			unit = fmt.Sprintf("per %d-command batch", cfg.pipeline)
		}
		fmt.Printf("\nlatency (%s)\n", unit)
		fmt.Printf("  p50 : %s\n", pctl(s.latencies, 0.50).Round(time.Microsecond))
		fmt.Printf("  p90 : %s\n", pctl(s.latencies, 0.90).Round(time.Microsecond))
		fmt.Printf("  p99 : %s\n", pctl(s.latencies, 0.99).Round(time.Microsecond))
		fmt.Printf("  max : %s\n", s.latencies[len(s.latencies)-1].Round(time.Microsecond))
	}

	if sample, ok := s.errSample.Load().(string); ok && sample != "" {
		fmt.Printf("\nfirst error: %s\n", sample)
		if s.appErrs > 0 {
			fmt.Printf("(hint: \"ERR\" replies usually mean the target isn't the leader — see bench/main.go docs)\n")
		}
	}

	if total != int64(cfg.requests) {
		fmt.Printf("\nnote: accounted for %d of %d planned requests (a connection died early)\n", total, cfg.requests)
	}
}

// pctl returns the q-quantile (0..1) of a pre-sorted slice using nearest-rank.
func pctl(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := max(int(q*float64(len(sorted)-1)), 0)
	return sorted[idx]
}
