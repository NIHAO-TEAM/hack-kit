package analytics

import (
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// ListCustomers — GET /api/v1/customers.
func (*AnalyticsAPI) ListCustomers(w responseWriter, r *request, _ gen.ListCustomersParams) {
	httpx.NotImplemented(w, r)
}

// GetCustomer — GET /api/v1/customers/{id}.
func (*AnalyticsAPI) GetCustomer(w responseWriter, r *request, _ gen.CustomerId) {
	httpx.NotImplemented(w, r)
}
