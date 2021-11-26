package selector

import (
	"context"
	"regexp"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

type (
	transporter     func(ctx context.Context) (transport.Transporter, bool)
	MatchFunc       func(operation string) bool
	MatchHeaderFunc func(headers map[string]string) bool
)

var (
	// serverTransporter is get server transport.Transporter from ctx
	serverTransporter transporter = func(ctx context.Context) (transport.Transporter, bool) {
		return transport.FromServerContext(ctx)
	}
	// clientTransporter is get client transport.Transporter from ctx
	clientTransporter transporter = func(ctx context.Context) (transport.Transporter, bool) {
		return transport.FromClientContext(ctx)
	}
)

// Builder is a selector builder
type Builder struct {
	client bool

	prefix      []string
	regex       []string
	path        []string
	match       MatchFunc
	matchHeader MatchHeaderFunc

	ms []middleware.Middleware
}

// Server selector middleware
func Server(ms ...middleware.Middleware) *Builder {
	return &Builder{ms: ms}
}

// Client selector middleware
func Client(ms ...middleware.Middleware) *Builder {
	return &Builder{client: true, ms: ms}
}

// Prefix is with Builder's prefix
func (b *Builder) Prefix(prefix ...string) *Builder {
	b.prefix = prefix
	return b
}

// Regex is with Builder's regex
func (b *Builder) Regex(regex ...string) *Builder {
	b.regex = regex
	return b
}

// Path is with Builder's path
func (b *Builder) Path(path ...string) *Builder {
	b.path = path
	return b
}

// Match is with Builder's match
func (b *Builder) Match(fn MatchFunc) *Builder {
	b.match = fn
	return b
}

// MatchHeader is with Builder's matchHeader
func (b *Builder) MatchHeader(fn MatchHeaderFunc) *Builder {
	b.matchHeader = fn
	return b
}

// Build is Builder's Build, for example: Server().Path(m1,m2).Build()
func (b *Builder) Build() middleware.Middleware {
	var transporter func(ctx context.Context) (transport.Transporter, bool)
	if b.client {
		transporter = clientTransporter
	} else {
		transporter = serverTransporter
	}
	return selector(transporter, b.matchs, b.ms...)
}

// matchs is match operation compliance Builder
func (b *Builder) matchs(tr transport.Transporter) bool {
	operation := tr.Operation()
	for _, prefix := range b.prefix {
		if prefixMatch(prefix, operation) {
			return true
		}
	}
	for _, regex := range b.regex {
		if regexMatch(regex, operation) {
			return true
		}
	}
	for _, path := range b.path {
		if pathMatch(path, operation) {
			return true
		}
	}

	if b.match != nil {
		if b.match(operation) {
			return true
		}
	}
	if b.matchHeader != nil {
		headers := make(map[string]string, len(tr.RequestHeader().Keys()))
		for _, k := range tr.RequestHeader().Keys() {
			headers[k] = tr.RequestHeader().Get(k)
		}
		if b.matchHeader(headers) {
			return true
		}
	}

	return false
}

// selector middleware
func selector(transporter transporter, match func(transport.Transporter) bool, ms ...middleware.Middleware) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			info, ok := transporter(ctx)
			if !ok {
				return handler(ctx, req)
			}

			if !match(info) {
				return handler(ctx, req)
			}
			return middleware.Chain(ms...)(handler)(ctx, req)
		}
	}
}

func pathMatch(path string, operation string) bool {
	return path == operation
}

func prefixMatch(prefix string, operation string) bool {
	return strings.HasPrefix(operation, prefix)
}

func regexMatch(regex string, operation string) bool {
	r, err := regexp.Compile(regex)
	if err != nil {
		return false
	}
	return r.FindString(operation) == operation
}
