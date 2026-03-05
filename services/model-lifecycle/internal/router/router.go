package router

import (
	"math/rand"
	"sync"
)

// Route defines traffic routing between model versions.
type Route struct {
	ModelA     string
	ModelB     string
	BTrafficPct int // 0-100: percentage of traffic routed to B
}

// Router manages A/B traffic routing.
type Router struct {
	mu    sync.RWMutex
	route *Route
}

// NewRouter creates a router with all traffic to model A.
func NewRouter(modelA string) *Router {
	return &Router{
		route: &Route{
			ModelA:      modelA,
			BTrafficPct: 0,
		},
	}
}

// SetABTest configures A/B routing.
func (r *Router) SetABTest(modelB string, bPct int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.route.ModelB = modelB
	r.route.BTrafficPct = bPct
}

// ClearABTest removes the B model and routes all traffic to A.
func (r *Router) ClearABTest() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.route.ModelB = ""
	r.route.BTrafficPct = 0
}

// RouteRequest returns which model should handle the request.
func (r *Router) RouteRequest() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.route.ModelB == "" || r.route.BTrafficPct == 0 {
		return r.route.ModelA
	}

	if rand.Intn(100) < r.route.BTrafficPct {
		return r.route.ModelB
	}
	return r.route.ModelA
}

// GetRoute returns the current routing config.
func (r *Router) GetRoute() Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return *r.route
}

// PromoteB makes model B the new A and clears B.
func (r *Router) PromoteB() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.route.ModelB != "" {
		r.route.ModelA = r.route.ModelB
		r.route.ModelB = ""
		r.route.BTrafficPct = 0
	}
}
