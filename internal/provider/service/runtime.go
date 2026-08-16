package service

import (
	"github.com/amigoer/fluxa/internal/provider/health"
	"github.com/amigoer/fluxa/internal/provider/keyauth"
	"github.com/amigoer/fluxa/internal/provider/routing"
)

// Runtime hands out the three long-lived collaborators the gateway hot
// path drives directly rather than through a per-call service method:
// they carry state of their own (the breaker's state machine, the key
// cache's TTL window) that a stateless wrapper would hide, and every
// request touches all three.
type Runtime interface {
	Breaker() *health.Breaker
	Keys() *keyauth.Cache
	Resolver() *routing.Resolver
}

func (s *service) Breaker() *health.Breaker { return s.breaker }

func (s *service) Keys() *keyauth.Cache { return s.keys }

func (s *service) Resolver() *routing.Resolver { return s.resolver }
