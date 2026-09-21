package providers

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

const bnrTestXML = `<DataSet xmlns="https://www.bnr.ro/xsd"><Body><Cube date="2026-09-18"><Rate currency="EUR">5.2644</Rate><Rate currency="USD">4.5839</Rate><Rate currency="GBP">6.1267</Rate><Rate currency="CHF">5.5564</Rate></Cube></Body></DataSet>`

type bnrRoundTripFunc func(*http.Request) (*http.Response, error)

func (f bnrRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type bnrTestBody struct {
	io.Reader
	err    error
	closed bool
}

func (b *bnrTestBody) Read(payload []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	return b.Reader.Read(payload)
}

func (b *bnrTestBody) Close() error {
	b.closed = true
	return nil
}

func TestBNRProviderFetchRetriesTransientFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		status  int
		bodyErr error
	}{
		{name: "connection reset", err: &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}},
		{name: "TLS handshake timeout", err: errors.New("net/http: TLS handshake timeout")},
		{name: "service unavailable", status: http.StatusServiceUnavailable},
		{name: "request timeout", status: http.StatusRequestTimeout},
		{name: "interrupted body", status: http.StatusOK, bodyErr: io.ErrUnexpectedEOF},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			failedBody := &bnrTestBody{Reader: strings.NewReader("unavailable"), err: test.bodyErr}
			successBody := &bnrTestBody{Reader: strings.NewReader(bnrTestXML)}
			provider := NewBNRProvider(&http.Client{Transport: bnrRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.URL.String() != BNRURL || request.Method != http.MethodGet {
					t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
				}
				if request.Header.Get("User-Agent") == "" || !strings.Contains(request.Header.Get("Accept"), "xml") {
					t.Fatal("BNR request must identify the application and accept XML")
				}
				deadline, ok := request.Context().Deadline()
				if !ok || time.Until(deadline) > bnrAttemptTimeout {
					t.Fatal("each attempt must leave time for retries within the collection deadline")
				}
				if calls == 1 {
					if test.err != nil {
						return nil, test.err
					}
					return &http.Response{StatusCode: test.status, Body: failedBody}, nil
				}
				if test.err == nil && !failedBody.closed {
					t.Fatal("failed response must be closed before retrying")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: successBody}, nil
			})})
			started := time.Now()
			snapshots, err := provider.Fetch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 || len(snapshots) != 4 || !successBody.closed {
				t.Fatalf("expected recovery on second attempt and a closed response: calls=%d, snapshots=%d, closed=%t", calls, len(snapshots), successBody.closed)
			}
			for _, snapshot := range snapshots {
				if snapshot.SourceURL != BNRURL || snapshot.FetchedAt.Before(started) || snapshot.EffectiveAt.Format("2006-01-02") != "2026-09-18" {
					t.Fatalf("recovery must preserve BNR provenance and publication date: %+v", snapshot)
				}
			}
		})
	}
}

func TestBNRProviderFetchLimitsRetries(t *testing.T) {
	calls := 0
	reset := errors.New("connection reset by peer")
	provider := NewBNRProvider(&http.Client{Transport: bnrRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, reset
	})})
	snapshots, err := provider.Fetch(context.Background())
	if calls != bnrFetchAttempts || snapshots != nil || !errors.Is(err, reset) {
		t.Fatalf("expected bounded failure without new rates: calls=%d, snapshots=%v, error=%v", calls, snapshots, err)
	}
	if !strings.Contains(err.Error(), "attempt 3/3") {
		t.Fatalf("error must report exhausted attempts: %v", err)
	}
}

func TestBNRProviderFetchDoesNotRetryPermanentFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "not found", status: http.StatusNotFound, body: "not found"},
		{name: "rate limited", status: http.StatusTooManyRequests, body: "slow down"},
		{name: "HTML instead of XML", status: http.StatusOK, body: "<html><body>Unavailable</body></html>"},
		{name: "incomplete currencies", status: http.StatusOK, body: `<DataSet><Body><Cube date="2026-09-18"><Rate currency="EUR">5.2644</Rate></Cube></Body></DataSet>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			body := &bnrTestBody{Reader: strings.NewReader(test.body)}
			provider := NewBNRProvider(&http.Client{Transport: bnrRoundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: test.status, Body: body}, nil
			})})
			snapshots, err := provider.Fetch(context.Background())
			if err == nil || snapshots != nil || calls != 1 || !body.closed {
				t.Fatalf("expected immediate rejection: calls=%d, snapshots=%v, error=%v, closed=%t", calls, snapshots, err, body.closed)
			}
		})
	}
}

func TestBNRProviderFetchRetriesClientTimeout(t *testing.T) {
	calls := 0
	provider := NewBNRProvider(&http.Client{Timeout: 10 * time.Millisecond, Transport: bnrRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(bnrTestXML))}, nil
	})})
	snapshots, err := provider.Fetch(context.Background())
	if err != nil || len(snapshots) != 4 || calls != 2 {
		t.Fatalf("one timed-out attempt must not cancel later attempts: calls=%d, snapshots=%d, error=%v", calls, len(snapshots), err)
	}
}

func TestBNRProviderFetchRespectsCancellation(t *testing.T) {
	for _, cancelBeforeRequest := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		provider := NewBNRProvider(&http.Client{Transport: bnrRoundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			cancel()
			return nil, errors.New("connection reset by peer")
		})})
		if cancelBeforeRequest {
			cancel()
		}
		snapshots, err := provider.Fetch(ctx)
		cancel()
		if snapshots != nil || !errors.Is(err, context.Canceled) || calls > 1 || (cancelBeforeRequest && calls != 0) {
			t.Fatalf("must stop on cancellation: calls=%d, snapshots=%v, error=%v", calls, snapshots, err)
		}
	}
}

func TestBNRProviderTLSCompatibilityPreservesSharedClient(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, bnrTestXML)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		if slices.Contains(hello.SupportedCurves, tls.X25519MLKEM768) {
			return nil, errors.New("server cannot handle hybrid key shares")
		}
		return nil, nil
	}}
	server.StartTLS()
	defer server.Close()

	client := server.Client()
	client.Timeout = time.Second
	sharedTransport := client.Transport.(*http.Transport)
	curves := []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256, tls.CurveP384}
	sharedTransport.TLSClientConfig.CurvePreferences = slices.Clone(curves)
	if response, err := client.Get(server.URL); err == nil {
		response.Body.Close()
		t.Fatal("test server must reject the default hybrid handshake")
	}
	provider := NewBNRProvider(client)
	defer provider.client.CloseIdleConnections()
	response, err := provider.client.Get(server.URL)
	if err != nil {
		t.Fatalf("BNR client must interoperate with servers that reject hybrid key shares: %v", err)
	}
	response.Body.Close()
	if client.Transport != sharedTransport || !slices.Equal(sharedTransport.TLSClientConfig.CurvePreferences, curves) || provider.client.Timeout != client.Timeout {
		t.Fatal("BNR compatibility settings must preserve the shared client and its timeout")
	}

	untrusted := NewBNRProvider(&http.Client{Timeout: time.Second})
	defer untrusted.client.CloseIdleConnections()
	response, err = untrusted.client.Get(server.URL)
	if err == nil {
		response.Body.Close()
		t.Fatal("BNR client must still reject untrusted certificates")
	}
	var verificationError *tls.CertificateVerificationError
	if !errors.As(err, &verificationError) {
		t.Fatalf("expected certificate verification failure, got %v", err)
	}
}

func TestParseBNRXMLAppliesMultiplier(t *testing.T) {
	input := `<DataSet><Body><Cube date="2026-08-19"><Rate currency="EUR" multiplier="100">523.4500</Rate><Rate currency="USD">4.5000</Rate><Rate currency="GBP">6.0000</Rate><Rate currency="CHF">5.5000</Rate></Cube></Body></DataSet>`
	snapshots, err := ParseBNRXML([]byte(input), time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshots[0].BuyRate.StringFixed(4); got != "5.2345" {
		t.Fatalf("expected multiplier-normalized EUR rate, got %s", got)
	}
}
