package connectors

import "context"

type Repository interface {
	List(ctx context.Context) ([]Connector, error)
	Get(ctx context.Context, id string) (Connector, error)
	Save(ctx context.Context, connector Connector) error
	Delete(ctx context.Context, id string) error
}
