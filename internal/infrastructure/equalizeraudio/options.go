package equalizeraudio

type TargetProcessProvider func() uint32

type Options struct {
	TargetProcessProvider TargetProcessProvider
}

type Option func(*Options)

func WithTargetProcessProvider(provider TargetProcessProvider) Option {
	return func(options *Options) {
		options.TargetProcessProvider = provider
	}
}

func resolveOptions(options []Option) Options {
	var resolved Options
	for _, option := range options {
		if option != nil {
			option(&resolved)
		}
	}
	return resolved
}
