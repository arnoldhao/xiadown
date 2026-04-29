package dependencies

import "context"

type Repository interface {
	List(ctx context.Context) ([]Dependency, error)
	Get(ctx context.Context, name string) (Dependency, error)
	Save(ctx context.Context, tool Dependency) error
	Delete(ctx context.Context, name string) error
}
