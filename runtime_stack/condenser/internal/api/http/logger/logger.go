package logger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	ctxEventKey ctxKey = iota
)

func LoggerMiddleware(l Logger, component string, node string) func(http.Handler) http.Handler {
	if component == "" {
		component = "condenser"
	}
	index := make(map[string]Rule, len(rules))
	for _, ru := range rules {
		key := ru.Method + " " + ru.Pattern
		index[key] = ru
	}
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			ev := Event{
				TS:            time.Now().Format(time.RFC3339Nano),
				EventId:       uuid.NewString(),
				CorrelationId: middleware.GetReqID(r.Context()),

				Severity: Severity[SEV_INFO],

				Actor: Actor{
					SPIFFEId:        spiffeIdFromRequest(r),
					CertFingerprint: clientCertFingerprint(r),
					PeerIp:          peerIp(r),
				},

				Request: Request{
					Method: r.Method,
					Path:   r.URL.Path,
					Host:   r.Host,
				},

				Result: Result{},

				Runtime: Runtime{
					Component: component,
					Node:      node,
				},

				Extra: map[string]any{},
			}

			ctx := context.WithValue(r.Context(), ctxEventKey, &ev)
			r = r.WithContext(ctx)

			next.ServeHTTP(ww, r)

			pattern := chi.RouteContext(r.Context()).RoutePattern()
			key := r.Method + " " + pattern
			if ev.Action == "" {
				if ru, ok := index[key]; ok {
					ev.Action = ru.Action
					ev.Severity = Severity[ru.Severity]
				} else {
					ev.Action = "unknown"
					ev.Severity = Severity[SEV_LOW]
				}
			} else {
				ev.Severity = Severity[severityForAction(ev.Action)]
			}

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			ev.Result.Code = status
			ev.Result.Bytes = ww.BytesWritten()
			ev.Result.LatencyMs = time.Since(start).Milliseconds()

			switch {
			case status >= 200 && status < 400:
				ev.Result.Status = "allow"
			case status == http.StatusUnauthorized || status == http.StatusForbidden:
				ev.Result.Status = "deny"
				ev.Severity = bump(ev.Severity)
			default:
				ev.Result.Status = "error"
				ev.Severity = bump(ev.Severity)
			}

			l.Write(ev)
		}
		return http.HandlerFunc(fn)
	}
}

func FromContext(ctx context.Context) *Event {
	ev, _ := ctx.Value(ctxEventKey).(*Event)
	return ev
}

func SetAction(ctx context.Context, action string) {
	if ev := FromContext(ctx); ev != nil {
		ev.Action = action
	}
}

func SetSevirity(ctx context.Context, sev int) {
	if ev := FromContext(ctx); ev != nil {
		ev.Severity = Severity[sev]
	}
}

func SetTarget(ctx context.Context, target Target) {
	if ev := FromContext(ctx); ev != nil {
		// container
		if target.ContainerId != "" {
			ev.Target.ContainerId = target.ContainerId
		}
		if target.ContainerName != "" {
			ev.Target.ContainerName = target.ContainerName
		}
		if target.Network != "" {
			ev.Target.Network = target.Network
		}
		if target.Tty {
			ev.Target.Tty = true
		}
		if target.ImageRef != "" {
			ev.Target.ImageRef = target.ImageRef
		}
		if len(target.Command) != 0 {
			ev.Target.Command = target.Command
		}
		if len(target.Port) != 0 {
			ev.Target.Port = target.Port
		}
		if len(target.Mount) != 0 {
			ev.Target.Mount = target.Mount
		}

		// policy
		if target.PolicyId != "" {
			ev.Target.PolicyId = target.PolicyId
		}
		if target.ChainName != "" {
			ev.Target.ChainName = target.ChainName
		}
		if target.Source != "" {
			ev.Target.Source = target.Source
		}
		if target.Destination != "" {
			ev.Target.Destination = target.Destination
		}
		if target.Protocol != "" {
			ev.Target.Protocol = target.Protocol
		}
		if target.DestPort > 0 {
			ev.Target.DestPort = target.DestPort
		}
		if target.Comment != "" {
			ev.Target.Comment = target.Comment
		}

		// pki
		if target.CommonName != "" {
			ev.Target.CommonName = target.CommonName
		}
		if len(target.SANURIs) > 0 {
			ev.Target.SANURIs = target.SANURIs
		}
	}
}

func SetReason(ctx context.Context, reason string) {
	if ev := FromContext(ctx); ev != nil {
		ev.Result.Reasone = reason
	}
}

func PutExtra(ctx context.Context, k string, v any) {
	if ev := FromContext(ctx); ev != nil {
		if ev.Extra == nil {
			ev.Extra = map[string]any{}
		}
		ev.Extra[k] = v
	}
}

type JsonLineLogger struct {
	Out ioWriter
}

type ioWriter interface {
	Write(p []byte) (n int, err error)
}

func (l JsonLineLogger) Write(event Event) {
	b, _ := json.Marshal(event)
	_, _ = l.Out.Write(append(b, '\n'))
}

func spiffeIdFromRequest(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return ""
	}
	leaf := r.TLS.PeerCertificates[0]
	if len(leaf.URIs) == 0 {
		return ""
	}
	return leaf.URIs[0].String()
}

func clientCertFingerprint(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return ""
	}
	leaf := r.TLS.PeerCertificates[0]
	sum := sha256.Sum256(leaf.Raw)
	return hex.EncodeToString(sum[:])
}

func peerIp(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func severityForAction(action string) int {
	if s, ok := actionSeverity[action]; ok {
		return s
	}
	return SEV_LOW
}

func bump(s string) string {
	switch s {
	case "information":
		return "low"
	case "low":
		return "medium"
	case "medium":
		return "high"
	case "high":
		return "critical"
	default:
		return s
	}
}
