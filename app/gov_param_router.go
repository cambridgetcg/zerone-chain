package app

import (
	"context"
	"fmt"

	govtypes "github.com/zerone-chain/zerone/x/gov/types"
)

// ParamChangeHandler applies a single parameter change for a module.
// key is the parameter name, value is the new value as a string.
type ParamChangeHandler func(ctx context.Context, key, value string) error

// GovParamRouter dispatches parameter changes from passed LIPs to the
// appropriate module keeper. Modules register handlers via Register.
type GovParamRouter struct {
	handlers map[string]ParamChangeHandler
}

// NewGovParamRouter creates a new empty param router.
func NewGovParamRouter() *GovParamRouter {
	return &GovParamRouter{
		handlers: make(map[string]ParamChangeHandler),
	}
}

// Register adds a handler for the given module name.
func (r *GovParamRouter) Register(module string, handler ParamChangeHandler) {
	r.handlers[module] = handler
}

// ApplyParamChange routes a parameter change to the registered module handler.
func (r *GovParamRouter) ApplyParamChange(ctx context.Context, module, key, value string) error {
	handler, ok := r.handlers[module]
	if !ok {
		return fmt.Errorf("no param handler registered for module %q", module)
	}
	return handler(ctx, key, value)
}

var _ govtypes.ParamRouter = (*GovParamRouter)(nil)
