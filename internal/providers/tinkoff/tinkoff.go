package tinkoff

import (
	"fmt"
	sdk "github.com/TinkoffCreditSystems/invest-openapi-go-sdk"
	"go.uber.org/zap"
	"goinvest/internal/invest"
)

type providerTinkoff struct {
	logger        *zap.Logger
	cache         invest.Cache
	sandboxClient *sdk.SandboxRestClient
	client        *sdk.RestClient
}

// ProviderOptions represents optional fields while constructing Tinkoff.
type ProviderOptions struct {
	// Client represents library client used for communication with Tinkoff.
	// Optional.
	SandboxClient     *sdk.SandboxRestClient
	Client            *sdk.RestClient
	Token             string
	SandboxToken      string
	RequestsPerSecond int
}

func NewTinkoff(opts *ProviderOptions, cache invest.Cache, logger *zap.Logger) (invest.Provider, error) {

	if logger == nil {
		return nil, fmt.Errorf("provider %s: logger must be provided", "tinkoff")
	}

	if cache == nil {
		return nil, fmt.Errorf("provider %s: cache must be provided", "tinkoff")
	}
	var (
		client        *sdk.RestClient
		sandboxClient *sdk.SandboxRestClient
	)
	if opts != nil {
		if opts.SandboxClient != nil {
			sandboxClient = opts.SandboxClient
		} else {
			sandboxClient = sdk.NewSandboxRestClient(opts.SandboxToken)
		}
		if opts.Client != nil {
			client = opts.Client
		} else {
			client = sdk.NewRestClient(opts.Token)
		}
	}

	return &providerTinkoff{
		logger:        logger,
		cache:         cache,
		client:        client,
		sandboxClient: sandboxClient,
	}, nil
}
