//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// rbHTTPCanonKey is MRI's Net::HTTPHeader/WEBrick key form: lowercase, unlike Go's Title-Case canonical form.
func rbHTTPCanonKey(name string) string { return strings.ToLower(name) }

// rbHTTPHeaderHash turns a Go http.Header into the insertion-ordered, lowercase-keyed Hash Net::HTTPHeader/WEBrick expose; sorted so output is deterministic.
func rbHTTPHeaderHash(h http.Header) *Hash[String, String] {
	out := NewHash[String, String]()
	for _, k := range slices.Sorted(maps.Keys(h)) {
		Hash_Op_idxSet(out, String(rbHTTPCanonKey(k)), String(strings.Join(h[k], ", ")))
	}
	return out
}

// rbHTTPHeaderFields is rbHTTPHeaderHash keeping repeated headers apart, as Net::HTTPHeader#to_hash does.
func rbHTTPHeaderFields(h http.Header) *Hash[String, *Array[String]] {
	out := NewHash[String, *Array[String]]()
	for _, k := range slices.Sorted(maps.Keys(h)) {
		vals := &Array[String]{s: make([]String, 0, len(h[k]))}
		for _, v := range h[k] {
			vals.s = append(vals.s, String(v))
		}
		Hash_Op_idxSet(out, String(rbHTTPCanonKey(k)), vals)
	}
	return out
}

// rbHTTPOpenErr tags a dial-phase failure so rbHTTPRequest can tell Net::OpenTimeout from Net::ReadTimeout.
type rbHTTPOpenErr struct {
	error
	addr string
}

func (e *rbHTTPOpenErr) Unwrap() error { return e.error }

// rbHTTPClientFor builds a client per h's use_ssl/open_timeout/read_timeout; open_timeout bounds dialing, read_timeout the rest of the round trip.
func rbHTTPClientFor(h *Net_HTTP) *http.Client {
	open := time.Duration(float64(h.openTimeout) * float64(time.Second))
	read := time.Duration(float64(h.readTimeout) * float64(time.Second))
	dialer := &net.Dialer{Timeout: open}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, &rbHTTPOpenErr{err, addr}
			}
			return conn, nil
		},
	}
	if h.verifyMode != nil && *h.verifyMode == 0 { // OpenSSL::SSL::VERIFY_NONE
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // the caller asked for VERIFY_NONE
	}
	return &http.Client{
		Transport:     transport,
		Timeout:       open + read,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// rbHTTPFailure is the Ruby exception for a failed round trip; Client.Timeout covers dialing, headers and the body read, so one classification serves both.
func rbHTTPFailure(err error) any {
	var openErr *rbHTTPOpenErr
	switch {
	case errors.As(err, &openErr):
		return rbHTTPOpenFailure(openErr)
	case errors.Is(err, context.DeadlineExceeded):
		return NewNet_ReadTimeout(Ref(String("execution expired")))
	default:
		return NewIOError(Ref(String(err.Error())))
	}
}

// rbHTTPOpenFailure is MRI's Errno for a refused or unreachable dial (Net::HTTP#connect's wording), OpenTimeout for a timeout.
func rbHTTPOpenFailure(e *rbHTTPOpenErr) any {
	var errno syscall.Errno
	if errors.As(e.error, &errno) && !errors.Is(e.error, os.ErrDeadlineExceeded) {
		if f, ok := rbSockErrnos[errno]; ok {
			host, port, _ := net.SplitHostPort(e.addr)
			s := errno.Error()
			s = strings.ToUpper(s[:1]) + s[1:]
			return f(Ref(String(fmt.Sprintf("Failed to open TCP connection to %s (%s - connect(2) for %q port %s)", e.addr, s, host, port))))
		}
	}
	return NewNet_OpenTimeout(Ref(String("execution expired")))
}

func rbHTTPRequest(h *Net_HTTP, method, path string, body *String, headers *Hash[String, String]) Net_HTTPResponseI {
	if path == "" {
		panic(NewArgumentError(Ref(String("HTTP request path is empty"))))
	}
	var rd io.Reader
	if body != nil {
		rd = strings.NewReader(string(*body))
	}
	scheme := "http"
	if h.useSSL {
		scheme = "https"
	}
	// MRI sends the path as the request-target verbatim, so it can never name the host: a path without a leading "/" ("@evil.com/x" would turn host:port into userinfo) is parsed behind one and the slash dropped again.
	base := scheme + "://" + net.JoinHostPort(string(h.address), strconv.Itoa(int(h.port)))
	noSlash := path[0] != '/'
	if noSlash {
		path = "/" + path
	}
	target, err := url.Parse(base + path)
	if err != nil {
		panic(NewArgumentError(Ref(String(err.Error()))))
	}
	req, err := http.NewRequestWithContext(context.Background(), method, base, rd)
	if err != nil {
		panic(NewArgumentError(Ref(String(err.Error()))))
	}
	req.URL.Path, req.URL.RawPath, req.URL.RawQuery = target.Path, target.RawPath, target.RawQuery
	if noSlash {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/")
		req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/")
	}
	req.Header.Set("User-Agent", "Ruby")
	for k, hv := range rbHashPairs(headers) {
		req.Header.Set(string(k), string(hv))
	}
	client := rbHTTPClientFor(h)
	// The transport is per request: closing its idle connections (also ones still on their way to idle) is what frees the fd and goroutines, without a Connection: close header.
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		panic(rbHTTPFailure(err))
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := rbReadAllLimit(resp.Body, rbHTTPMaxBody)
	if errors.Is(err, errRBBodyTooLarge) {
		panic(NewNoMemoryError(Ref(String("Net::HTTP response body too large"))))
	}
	if err != nil {
		panic(rbHTTPFailure(err))
	}
	return rbHTTPResponseFor(strconv.Itoa(resp.StatusCode), http.StatusText(resp.StatusCode), string(data), rbHTTPHeaderHash(resp.Header), rbHTTPHeaderFields(resp.Header))
}

// rbHTTPResponseFor picks Net::HTTPResponse's subclass by status code, matching MRI's CODE_TO_OBJ (checked against `ruby -rnet/http -e 'p Net::HTTPResponse::CODE_TO_OBJ'`), falling back to the category by first digit and then HTTPUnknownResponse.
func rbHTTPResponseFor(code, message, body string, header *Hash[String, String], fields *Hash[String, *Array[String]]) Net_HTTPResponseI {
	c, m, b := String(code), String(message), String(body)
	switch code {
	case "100":
		return NewNet_HTTPContinue(c, m, b, header, fields)
	case "101":
		return NewNet_HTTPSwitchProtocol(c, m, b, header, fields)
	case "102":
		return NewNet_HTTPProcessing(c, m, b, header, fields)
	case "103":
		return NewNet_HTTPEarlyHints(c, m, b, header, fields)
	case "200":
		return NewNet_HTTPOK(c, m, b, header, fields)
	case "201":
		return NewNet_HTTPCreated(c, m, b, header, fields)
	case "202":
		return NewNet_HTTPAccepted(c, m, b, header, fields)
	case "203":
		return NewNet_HTTPNonAuthoritativeInformation(c, m, b, header, fields)
	case "204":
		return NewNet_HTTPNoContent(c, m, b, header, fields)
	case "205":
		return NewNet_HTTPResetContent(c, m, b, header, fields)
	case "206":
		return NewNet_HTTPPartialContent(c, m, b, header, fields)
	case "207":
		return NewNet_HTTPMultiStatus(c, m, b, header, fields)
	case "208":
		return NewNet_HTTPAlreadyReported(c, m, b, header, fields)
	case "226":
		return NewNet_HTTPIMUsed(c, m, b, header, fields)
	case "300":
		return NewNet_HTTPMultipleChoices(c, m, b, header, fields)
	case "301":
		return NewNet_HTTPMovedPermanently(c, m, b, header, fields)
	case "302":
		return NewNet_HTTPFound(c, m, b, header, fields)
	case "303":
		return NewNet_HTTPSeeOther(c, m, b, header, fields)
	case "304":
		return NewNet_HTTPNotModified(c, m, b, header, fields)
	case "305":
		return NewNet_HTTPUseProxy(c, m, b, header, fields)
	case "307":
		return NewNet_HTTPTemporaryRedirect(c, m, b, header, fields)
	case "308":
		return NewNet_HTTPPermanentRedirect(c, m, b, header, fields)
	case "400":
		return NewNet_HTTPBadRequest(c, m, b, header, fields)
	case "401":
		return NewNet_HTTPUnauthorized(c, m, b, header, fields)
	case "402":
		return NewNet_HTTPPaymentRequired(c, m, b, header, fields)
	case "403":
		return NewNet_HTTPForbidden(c, m, b, header, fields)
	case "404":
		return NewNet_HTTPNotFound(c, m, b, header, fields)
	case "405":
		return NewNet_HTTPMethodNotAllowed(c, m, b, header, fields)
	case "406":
		return NewNet_HTTPNotAcceptable(c, m, b, header, fields)
	case "407":
		return NewNet_HTTPProxyAuthenticationRequired(c, m, b, header, fields)
	case "408":
		return NewNet_HTTPRequestTimeout(c, m, b, header, fields)
	case "409":
		return NewNet_HTTPConflict(c, m, b, header, fields)
	case "410":
		return NewNet_HTTPGone(c, m, b, header, fields)
	case "411":
		return NewNet_HTTPLengthRequired(c, m, b, header, fields)
	case "412":
		return NewNet_HTTPPreconditionFailed(c, m, b, header, fields)
	case "413":
		return NewNet_HTTPPayloadTooLarge(c, m, b, header, fields)
	case "414":
		return NewNet_HTTPURITooLong(c, m, b, header, fields)
	case "415":
		return NewNet_HTTPUnsupportedMediaType(c, m, b, header, fields)
	case "416":
		return NewNet_HTTPRangeNotSatisfiable(c, m, b, header, fields)
	case "417":
		return NewNet_HTTPExpectationFailed(c, m, b, header, fields)
	case "421":
		return NewNet_HTTPMisdirectedRequest(c, m, b, header, fields)
	case "422":
		return NewNet_HTTPUnprocessableEntity(c, m, b, header, fields)
	case "423":
		return NewNet_HTTPLocked(c, m, b, header, fields)
	case "424":
		return NewNet_HTTPFailedDependency(c, m, b, header, fields)
	case "426":
		return NewNet_HTTPUpgradeRequired(c, m, b, header, fields)
	case "428":
		return NewNet_HTTPPreconditionRequired(c, m, b, header, fields)
	case "429":
		return NewNet_HTTPTooManyRequests(c, m, b, header, fields)
	case "431":
		return NewNet_HTTPRequestHeaderFieldsTooLarge(c, m, b, header, fields)
	case "451":
		return NewNet_HTTPUnavailableForLegalReasons(c, m, b, header, fields)
	case "500":
		return NewNet_HTTPInternalServerError(c, m, b, header, fields)
	case "501":
		return NewNet_HTTPNotImplemented(c, m, b, header, fields)
	case "502":
		return NewNet_HTTPBadGateway(c, m, b, header, fields)
	case "503":
		return NewNet_HTTPServiceUnavailable(c, m, b, header, fields)
	case "504":
		return NewNet_HTTPGatewayTimeout(c, m, b, header, fields)
	case "505":
		return NewNet_HTTPVersionNotSupported(c, m, b, header, fields)
	case "506":
		return NewNet_HTTPVariantAlsoNegotiates(c, m, b, header, fields)
	case "507":
		return NewNet_HTTPInsufficientStorage(c, m, b, header, fields)
	case "508":
		return NewNet_HTTPLoopDetected(c, m, b, header, fields)
	case "510":
		return NewNet_HTTPNotExtended(c, m, b, header, fields)
	case "511":
		return NewNet_HTTPNetworkAuthenticationRequired(c, m, b, header, fields)
	}
	if len(code) == 0 {
		return NewNet_HTTPUnknownResponse(c, m, b, header, fields)
	}
	switch code[0] {
	case '1':
		return NewNet_HTTPInformation(c, m, b, header, fields)
	case '2':
		return NewNet_HTTPSuccess(c, m, b, header, fields)
	case '3':
		return NewNet_HTTPRedirection(c, m, b, header, fields)
	case '4':
		return NewNet_HTTPClientError(c, m, b, header, fields)
	case '5':
		return NewNet_HTTPServerError(c, m, b, header, fields)
	}
	return NewNet_HTTPUnknownResponse(c, m, b, header, fields)
}
