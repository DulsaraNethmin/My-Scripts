package catalog

import "context"

type contextKey struct{}

// NewContext returns ctx carrying cat, so commands can reach the catalog
// without a package-level variable.
func NewContext(ctx context.Context, cat *Catalog) context.Context {
	return context.WithValue(ctx, contextKey{}, cat)
}

// FromContext returns the catalog carried by ctx, if any.
func FromContext(ctx context.Context) (*Catalog, bool) {
	cat, ok := ctx.Value(contextKey{}).(*Catalog)
	return cat, ok
}
