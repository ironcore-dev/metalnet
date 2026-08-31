package key_exchange_client

import "context"

type KexExchangeClient interface {
	Init(ctx context.Context, clientName, clientPrefix string) error
	Subscribe(ctx context.Context, vni uint32) error
	Unsubscribe(ctx context.Context, vni uint32) error
	IsKeyReady(ctx context.Context, vni, timeout uint32) (bool, error)
}
