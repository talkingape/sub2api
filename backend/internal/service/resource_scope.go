package service

import "context"

type resourceManagerScopeKey struct{}

// ResourceManagerScope limits management queries without changing scheduler access.
type ResourceManagerScope struct {
	UserID                     int64
	IncludeContributableGroups bool
}

func WithResourceManagerScope(ctx context.Context, userID int64, includeContributableGroups bool) context.Context {
	return context.WithValue(ctx, resourceManagerScopeKey{}, ResourceManagerScope{userID, includeContributableGroups})
}

func ResourceManagerScopeFromContext(ctx context.Context) (ResourceManagerScope, bool) {
	scope, ok := ctx.Value(resourceManagerScopeKey{}).(ResourceManagerScope)
	return scope, ok
}

func normalizedManagerUserID(id *int64) *int64 {
	if id == nil || *id == 0 {
		return nil
	}
	return id
}

// OperatorResourcePatch records only explicitly supplied fields. Scoped updates
// must never persist policy or ownership fields from a previously loaded entity.
type OperatorResourcePatch struct {
	Name        *string
	Description *string
	Concurrency *int
	Priority    *int
}

type operatorResourcePatchKey struct{}

func WithOperatorResourcePatch(ctx context.Context, patch OperatorResourcePatch) context.Context {
	return context.WithValue(ctx, operatorResourcePatchKey{}, patch)
}

func OperatorResourcePatchFromContext(ctx context.Context) (OperatorResourcePatch, bool) {
	patch, ok := ctx.Value(operatorResourcePatchKey{}).(OperatorResourcePatch)
	return patch, ok
}

type resourceTransactionKey struct{}

type ResourceTransactionRunner interface {
	RunResourceTransaction(context.Context, func(context.Context) error) error
}

func WithResourceTransaction(ctx context.Context) context.Context {
	return context.WithValue(ctx, resourceTransactionKey{}, true)
}

func InResourceTransaction(ctx context.Context) bool {
	active, _ := ctx.Value(resourceTransactionKey{}).(bool)
	return active
}
