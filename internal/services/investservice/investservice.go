package investservice

import (
	"context"
	"errors"
	"go.uber.org/zap"
	pb "goinvest/gen/proto/go/invest/v1"
	"goinvest/internal/invest"
	"goinvest/internal/services/providerservice"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service which implements GRPC server.
type Service struct {
	pb.UnimplementedInvestServiceServer
	storage         invest.Storage
	cache           invest.Cache
	logger          *zap.Logger
	providerService *providerservice.ProviderService
}

func NewService(providerService *providerservice.ProviderService, storage invest.Storage, cache invest.Cache, logger *zap.Logger) (*Service, error) {

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

	return &Service{
		storage:         storage,
		cache:           cache,
		logger:          logger,
		providerService: providerService,
	}, nil
}

func (s *Service) GetAccounts(ctx context.Context, req *pb.AccountsRequest) (*pb.AccountsResponse, error) {
	return s.Provider().Accounts(ctx, req)
}

func (s *Service) GetPortfolio(ctx context.Context, req *pb.PortfolioRequest) (*pb.PortfolioResponse, error) {
	return s.Provider().Portfolio(ctx, req)
}

func (s *Service) Provider() invest.Provider {
	provider, err := s.providerService.Provider(invest.ProviderTinkoff)
	if err != nil {
		s.logger.Error("cannon get provider", zap.Error(err))
		return nil
	}
	return provider
}

// ValidationUnaryInterceptor validates incoming requests
func (s *Service) ValidationUnaryInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	if v, ok := req.(invest.Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}

	return handler(ctx, req)
}

// ErrorUnaryInterceptor intercepts known errors and returns the appropriate GRPC status code.
func (s *Service) ErrorUnaryInterceptor(
	ctx context.Context,
	req interface{},
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (resp interface{}, err error) {

	resp, err = handler(ctx, req)
	if err == nil {
		return
	}

	// errorsTotal.Inc()

	if errors.Is(err, invest.ErrNotFound) {
		err = status.Error(codes.NotFound, err.Error())
		return
	}

	err = status.Error(codes.Internal, err.Error())
	return
}
