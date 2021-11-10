package providerservice

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"goinvest/internal/invest"
	"goinvest/internal/providers/tinkoff"
)

// ProviderService is responsible for the choice of appropriate provider.
type ProviderService struct {
	conf            *invest.ProvidersConfig
	providerStorage invest.Storage
	cache           invest.Cache
	providers       map[invest.ProviderID]invest.Provider
	logger          *zap.Logger
}

// NewProviderService is a constructor-like function which constructs ProviderService, initializing providers under the hood.
func NewProviderService(
	conf *invest.ProvidersConfig,
	providerStorage invest.Storage,
	cache invest.Cache,
	logger *zap.Logger) (*ProviderService, error) {

	if conf == nil {
		return nil, errors.New("provider service: providers config passed to service is nil")
	}

	if providerStorage == nil {
		return nil, errors.New("provider service: provider storage provided to service is nil")
	}

	if cache == nil {
		return nil, errors.New("provider service: cache provided to service is nil")
	}

	if logger == nil {
		return nil, errors.New("provider service: logger provided to service is nil")
	}

	providerService := &ProviderService{
		conf:            conf,
		providerStorage: providerStorage,
		cache:           cache,
		logger:          logger,
	}

	if err := providerService.initProviders(); err != nil {
		return nil, err
	}

	return providerService, nil
}

// initProviders retrieves providers data from database, then constructs provider clients among with
// its options.
func (ps *ProviderService) initProviders() error {

	providersMap := make(map[invest.ProviderID]invest.Provider, 1)

	options := &tinkoff.ProviderOptions{
		Token:        ps.conf.Tinkoff.Token,
		SandboxToken: ps.conf.Tinkoff.TokenSandbox,
	}
	tinkoffProvider, err := tinkoff.NewTinkoff(options, ps.cache, ps.logger)
	if err != nil {
		return fmt.Errorf("problem with tinkoffProvider init: %w", err)
	}

	providersMap[invest.ProviderTinkoff] = tinkoffProvider

	ps.providers = providersMap

	return nil
}

// Provider is a getter which chooses provider from available provider map by provider id.
func (ps *ProviderService) Provider(providerID invest.ProviderID) (invest.Provider, error) {
	if provider, found := ps.providers[providerID]; found {
		return provider, nil
	}
	return nil, errors.New("provider was not found")
}
