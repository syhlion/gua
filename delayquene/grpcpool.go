package delayquene

import (
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// grpcClientPool caches one *grpc.ClientConn per delivery target so recurring
// jobs reuse a single HTTP/2 connection instead of dialling — and tearing the
// connection down — on every trigger. gRPC ClientConns are goroutine-safe and
// multiplex concurrent RPCs, so one per target is enough.
type grpcClientPool struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn
}

func newGrpcClientPool() *grpcClientPool {
	return &grpcClientPool{conns: make(map[string]*grpc.ClientConn)}
}

// conn returns a cached ClientConn for target, creating one on first use.
// grpc.NewClient is lazy — the actual transport is established on the first RPC
// (governed by that RPC's context), so this never blocks on the network.
func (p *grpcClientPool) conn(target string) (*grpc.ClientConn, error) {
	p.mu.RLock()
	c, ok := p.conns[target]
	p.mu.RUnlock()
	if ok {
		return c, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.conns[target]; ok { // re-check: another goroutine may have won
		return c, nil
	}
	c, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second, // ping idle conns to keep NAT/LB paths open
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, err
	}
	p.conns[target] = c
	return c, nil
}

// Close tears down every pooled connection. Called on queue shutdown.
func (p *grpcClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		_ = c.Close()
	}
	p.conns = make(map[string]*grpc.ClientConn)
}
