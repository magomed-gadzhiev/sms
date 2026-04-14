package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	companyv1 "github.com/smpp-server/smpp-server/api/proto/companyv1"
	"github.com/smpp-server/smpp-server/internal/services/company/application"
	"github.com/smpp-server/smpp-server/internal/services/company/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type Server struct {
	companyv1.UnimplementedCompanyServiceServer
	svc    *application.CompanyService
	logger zerolog.Logger
}

func NewServer(svc *application.CompanyService) *Server {
	return &Server{
		svc:    svc,
		logger: log.With().Str("component", "company-grpc-server").Logger(),
	}
}

func (s *Server) CreateCompany(ctx context.Context, req *companyv1.CreateCompanyRequest) (*companyv1.CompanyResponse, error) {
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	c, err := s.svc.CreateCompany(ctx, application.CreateCompanyInput{
		ClientID: clientID, INN: req.Inn, Name: req.Name, FullName: req.FullName,
		KPP: req.Kpp, OGRN: req.Ogrn, LegalAddress: req.LegalAddress,
		ActualAddress: req.ActualAddress, CEOName: req.CeoName, CEOTitle: req.CeoTitle,
		ActingBasis: req.ActingBasis, BankName: req.BankName, BankBIK: req.BankBik,
		BankCorrAccount: req.BankCorrAccount, BankAccount: req.BankAccount,
		Email: req.Email, Phone: req.Phone,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return &companyv1.CompanyResponse{Company: toProto(c, false)}, nil
}

func (s *Server) UpdateCompany(ctx context.Context, req *companyv1.UpdateCompanyRequest) (*companyv1.CompanyResponse, error) {
	if req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}
	companyID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}
	c, err := s.svc.UpdateCompany(ctx, application.UpdateCompanyInput{
		ID: companyID, INN: req.Inn, Name: req.Name, FullName: req.FullName,
		KPP: req.Kpp, OGRN: req.Ogrn, LegalAddress: req.LegalAddress,
		ActualAddress: req.ActualAddress, CEOName: req.CeoName, CEOTitle: req.CeoTitle,
		ActingBasis: req.ActingBasis, BankName: req.BankName, BankBIK: req.BankBik,
		BankCorrAccount: req.BankCorrAccount, BankAccount: req.BankAccount,
		Email: req.Email, Phone: req.Phone,
	})
	if err != nil {
		return nil, s.mapError(err)
	}
	return &companyv1.CompanyResponse{Company: toProto(c, false)}, nil
}

func (s *Server) GetCompany(ctx context.Context, req *companyv1.GetCompanyRequest) (*companyv1.CompanyResponse, error) {
	id, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}
	c, err := s.svc.GetCompany(ctx, id)
	if err != nil {
		return nil, s.mapError(err)
	}
	return &companyv1.CompanyResponse{Company: toProto(c, false)}, nil
}

func (s *Server) ListClientCompanies(ctx context.Context, req *companyv1.ListClientCompaniesRequest) (*companyv1.ListClientCompaniesResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	list, err := s.svc.ListClientCompanies(ctx, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	infos := make([]*companyv1.CompanyInfo, len(list))
	for i, cwd := range list {
		infos[i] = toProto(cwd.Company, cwd.IsDefault)
	}
	return &companyv1.ListClientCompaniesResponse{Companies: infos}, nil
}

func (s *Server) SetDefaultCompany(ctx context.Context, req *companyv1.SetDefaultCompanyRequest) (*emptypb.Empty, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	companyID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}
	if err := s.svc.SetDefaultCompany(ctx, clientID, companyID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) DetachCompany(ctx context.Context, req *companyv1.DetachCompanyRequest) (*emptypb.Empty, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	companyID, err := uuid.Parse(req.CompanyId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid company_id")
	}
	if err := s.svc.DetachCompany(ctx, clientID, companyID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) CreateOfferCompany(ctx context.Context, req *companyv1.CreateOfferCompanyRequest) (*companyv1.CompanyResponse, error) {
	clientID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid client_id")
	}
	c, err := s.svc.CreateOfferCompany(ctx, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return &companyv1.CompanyResponse{Company: toProto(c, true)}, nil
}

func (s *Server) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCompanyNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidINN),
		errors.Is(err, domain.ErrCompanyNameRequired):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrCompanyHasSenderNames),
		errors.Is(err, domain.ErrCannotDetachDefault):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrNotAttached):
		return status.Error(codes.NotFound, err.Error())
	default:
		s.logger.Error().Err(err).Msg("внутренняя ошибка company service")
		return status.Error(codes.Internal, "internal error")
	}
}

func toProto(c *shared.Company, isDefault bool) *companyv1.CompanyInfo {
	return &companyv1.CompanyInfo{
		Id:              c.ID.String(),
		Inn:             string(c.INN),
		Name:            c.Name,
		FullName:        string(c.FullName),
		Kpp:             string(c.KPP),
		Ogrn:            string(c.OGRN),
		LegalAddress:    string(c.LegalAddress),
		ActualAddress:   string(c.ActualAddress),
		CeoName:         string(c.CEOName),
		CeoTitle:        string(c.CEOTitle),
		ActingBasis:     string(c.ActingBasis),
		BankName:        string(c.BankName),
		BankBik:         string(c.BankBIK),
		BankCorrAccount: string(c.BankCorrAccount),
		BankAccount:     string(c.BankAccount),
		Email:           string(c.Email),
		Phone:           string(c.Phone),
		IsOffer:         c.IsOffer,
		IsDefault:       isDefault,
		Active:          c.Active,
		CreatedAt:       timestamppb.New(c.CreatedAt),
		UpdatedAt:       timestamppb.New(c.UpdatedAt),
	}
}
