package gqlservice

// THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.

import (
	"context"
	"errors"
	"go.uber.org/zap"
	gqlapi "goinvest/gen/gql/generated"
	gqlmodels "goinvest/gen/gql/models"
	pb "goinvest/gen/proto/go/invest/v1"
	"goinvest/internal/invest"
	"goinvest/internal/services/providerservice"
)

type Resolver struct {
	storage         invest.Storage
	cache           invest.Cache
	logger          *zap.Logger
	providerService *providerservice.ProviderService
}

func (r *mutationResolver) InvestServiceGetPortfolio(ctx context.Context, in *gqlmodels.PortfolioRequestInput) (*gqlmodels.PortfolioResponse, error) {
	portfolioPb, err := r.Provider().Portfolio(ctx, &pb.PortfolioRequest{Account: &pb.Account{
		AccountId: *in.Account.AccountID,
	}})
	if err != nil {
		return nil, err
	}
	positionsGql := convertPbPositionsToGql(portfolioPb.Positions)
	return &gqlmodels.PortfolioResponse{
		Positions: positionsGql,
	}, err
}

func (r *mutationResolver) InvestServiceGetAccounts(ctx context.Context) (*gqlmodels.AccountsResponse, error) {
	accountsPb, err := r.Provider().Accounts(ctx, &pb.AccountsRequest{})
	if err != nil {
		return nil, err
	}
	accountsGql := convertPbAccountsToGql(accountsPb.Accounts)
	return &gqlmodels.AccountsResponse{
		Accounts: accountsGql,
	}, err
}

func convertPbPositionsToGql(pbPositions []*pb.Position) []*gqlmodels.Position {
	gqlPosition := make([]*gqlmodels.Position, 0, len(pbPositions))
	for _, pbPosition := range pbPositions {
		lots := int(pbPosition.Lots)
		gqlPosition = append(gqlPosition, &gqlmodels.Position{
			Figi:           &pbPosition.Figi,
			Ticker:         &pbPosition.Ticker,
			Isin:           &pbPosition.Isin,
			InstrumentType: &pbPosition.InstrumentType,
			Balance:        &pbPosition.Balance,
			Blocked:        &pbPosition.Blocked,
			ExpectedYield: &gqlmodels.Yield{
				Currency: &pbPosition.ExpectedYield.Currency,
				Value:    &pbPosition.ExpectedYield.Value,
			},
			Lots: &lots,
			AveragePositionPrice: &gqlmodels.Yield{
				Currency: &pbPosition.AveragePositionPrice.Currency,
				Value:    &pbPosition.AveragePositionPrice.Value,
			},
			AveragePositionPriceNoNkd: &gqlmodels.Yield{
				Currency: &pbPosition.AveragePositionPriceNoNkd.Currency,
				Value:    &pbPosition.AveragePositionPriceNoNkd.Value,
			},
			Name: &pbPosition.Name,
		})
	}
	return gqlPosition
}

func convertPbAccountsToGql(pbAccounts []*pb.Account) []*gqlmodels.Account {
	gqlAccounts := make([]*gqlmodels.Account, 0, len(pbAccounts))
	var err error
	for _, pbAccount := range pbAccounts {
		var accountType gqlmodels.AccountType
		err = accountType.UnmarshalGQL(pbAccount.AccountType.String())
		if err != nil {
			accountType = gqlmodels.AccountTypeTypeUnspecified
		}
		gqlAccounts = append(gqlAccounts, &gqlmodels.Account{
			AccountID:   &pbAccount.AccountId,
			AccountType: &accountType,
		})
	}
	return gqlAccounts
}

func (r *mutationResolver) Provider() invest.Provider {
	provider, err := r.providerService.Provider(invest.ProviderTinkoff)
	if err != nil {
		r.logger.Error("cannon get provider", zap.Error(err))
		return nil
	}
	return provider
}

func (r *queryResolver) Dummy(ctx context.Context) (*bool, error) {
	panic("not implemented")
}

// Mutation returns gqlapi.MutationResolver implementation.
func (r *Resolver) Mutation() gqlapi.MutationResolver { return &mutationResolver{r} }

// Query returns gqlapi.QueryResolver implementation.
func (r *Resolver) Query() gqlapi.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }

func NewResolver(providerService *providerservice.ProviderService, storage invest.Storage, cache invest.Cache, logger *zap.Logger) (*Resolver, error) {

	if providerService == nil {
		return nil, errors.New("providerService provided to invest service is nil")
	}

	if storage == nil {
		return nil, errors.New("city storage provided to invest service is nil")
	}

	if cache == nil {
		return nil, errors.New("cache provided to invest service is nil")
	}

	if logger == nil {
		return nil, errors.New("logger provided to invest service is nil")
	}

	return &Resolver{
		storage:         storage,
		cache:           cache,
		logger:          logger,
		providerService: providerService,
	}, nil
}
