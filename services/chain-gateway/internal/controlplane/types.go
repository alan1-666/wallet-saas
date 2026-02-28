package controlplane

import "context"

type RPCEndpoint struct {
	ID        int64
	Chain     string
	Network   string
	Model     string
	URL       string
	Weight    int
	TimeoutMS int
	Priority  int
	Status    string
}

type RPCEndpointStore interface {
	ListActiveRPCEndpoints(ctx context.Context) ([]RPCEndpoint, error)
}
