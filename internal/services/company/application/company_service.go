package application

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/company/domain"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CompanyService реализует бизнес-логику работы с компаниями
type CompanyService struct {
	repo     CompanyRepository
	acctRepo AccountRepository
	logger   zerolog.Logger
}

// NewCompanyService создаёт новый сервис компаний
func NewCompanyService(repo CompanyRepository, acctRepo AccountRepository) *CompanyService {
	return &CompanyService{
		repo:     repo,
		acctRepo: acctRepo,
		logger:   log.With().Str("component", "company-service").Logger(),
	}
}

// CreateCompanyInput — входные данные для создания компании
type CreateCompanyInput struct {
	ClientID        uuid.UUID
	INN             string
	Name            string
	FullName        string
	KPP             string
	OGRN            string
	LegalAddress    string
	ActualAddress   string
	CEOName         string
	CEOTitle        string
	ActingBasis     string
	BankName        string
	BankBIK         string
	BankCorrAccount string
	BankAccount     string
	Email           string
	Phone           string
}

// CreateCompany создаёт новую компанию и привязывает её к клиенту
func (s *CompanyService) CreateCompany(ctx context.Context, in CreateCompanyInput) (*shared.Company, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, domain.ErrCompanyNameRequired
	}
	if !domain.ValidateINN(in.INN) {
		return nil, domain.ErrInvalidINN
	}

	c := &shared.Company{
		INN:             shared.NullString(in.INN),
		Name:            strings.TrimSpace(in.Name),
		FullName:        shared.NullString(in.FullName),
		KPP:             shared.NullString(in.KPP),
		OGRN:            shared.NullString(in.OGRN),
		LegalAddress:    shared.NullString(in.LegalAddress),
		ActualAddress:   shared.NullString(in.ActualAddress),
		CEOName:         shared.NullString(in.CEOName),
		CEOTitle:        shared.NullString(in.CEOTitle),
		ActingBasis:     shared.NullString(in.ActingBasis),
		BankName:        shared.NullString(in.BankName),
		BankBIK:         shared.NullString(in.BankBIK),
		BankCorrAccount: shared.NullString(in.BankCorrAccount),
		BankAccount:     shared.NullString(in.BankAccount),
		Email:           shared.NullString(in.Email),
		Phone:           shared.NullString(in.Phone),
		IsOffer:         false,
		Active:          true,
	}

	created, err := s.repo.Create(ctx, c)
	if err != nil {
		return nil, err
	}

	if err := s.repo.AttachCompany(ctx, in.ClientID, created.ID, false); err != nil {
		return nil, err
	}

	if s.acctRepo != nil {
		if err := s.acctRepo.CreateForCompany(ctx, in.ClientID, created.ID, "RUB"); err != nil {
			s.logger.Warn().Err(err).Msg("не удалось создать аккаунт для компании")
		}
	}

	return created, nil
}

// CreateOfferCompany создаёт системную компанию «Оферта» для нового клиента.
func (s *CompanyService) CreateOfferCompany(ctx context.Context, clientID uuid.UUID) (*shared.Company, error) {
	c := &shared.Company{
		Name:    "Оферта",
		IsOffer: true,
		Active:  true,
	}
	created, err := s.repo.Create(ctx, c)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AttachCompany(ctx, clientID, created.ID, true); err != nil {
		return nil, err
	}
	if s.acctRepo != nil {
		if err := s.acctRepo.CreateForCompany(ctx, clientID, created.ID, "RUB"); err != nil {
			s.logger.Warn().Err(err).Msg("не удалось создать аккаунт для компании «Оферта»")
		}
	}
	return created, nil
}

// GetCompany возвращает компанию по ID
func (s *CompanyService) GetCompany(ctx context.Context, id uuid.UUID) (*shared.Company, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateCompanyInput — входные данные для обновления компании
type UpdateCompanyInput struct {
	ID              uuid.UUID
	INN             string
	Name            string
	FullName        string
	KPP             string
	OGRN            string
	LegalAddress    string
	ActualAddress   string
	CEOName         string
	CEOTitle        string
	ActingBasis     string
	BankName        string
	BankBIK         string
	BankCorrAccount string
	BankAccount     string
	Email           string
	Phone           string
}

// UpdateCompany обновляет данные компании
func (s *CompanyService) UpdateCompany(ctx context.Context, in UpdateCompanyInput) (*shared.Company, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, domain.ErrCompanyNameRequired
	}
	if !domain.ValidateINN(in.INN) {
		return nil, domain.ErrInvalidINN
	}

	existing, err := s.repo.GetByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	existing.INN = shared.NullString(in.INN)
	existing.Name = strings.TrimSpace(in.Name)
	existing.FullName = shared.NullString(in.FullName)
	existing.KPP = shared.NullString(in.KPP)
	existing.OGRN = shared.NullString(in.OGRN)
	existing.LegalAddress = shared.NullString(in.LegalAddress)
	existing.ActualAddress = shared.NullString(in.ActualAddress)
	existing.CEOName = shared.NullString(in.CEOName)
	existing.CEOTitle = shared.NullString(in.CEOTitle)
	existing.ActingBasis = shared.NullString(in.ActingBasis)
	existing.BankName = shared.NullString(in.BankName)
	existing.BankBIK = shared.NullString(in.BankBIK)
	existing.BankCorrAccount = shared.NullString(in.BankCorrAccount)
	existing.BankAccount = shared.NullString(in.BankAccount)
	existing.Email = shared.NullString(in.Email)
	existing.Phone = shared.NullString(in.Phone)

	return s.repo.Update(ctx, existing)
}

// CompanyWithDefault — компания с флагом основной
type CompanyWithDefault struct {
	*shared.Company
	IsDefault bool
}

// ListClientCompanies возвращает все компании клиента с флагом основной
func (s *CompanyService) ListClientCompanies(ctx context.Context, clientID uuid.UUID) ([]*CompanyWithDefault, error) {
	companies, defaults, err := s.repo.ListByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	out := make([]*CompanyWithDefault, len(companies))
	for i, c := range companies {
		out[i] = &CompanyWithDefault{Company: c, IsDefault: defaults[i]}
	}
	return out, nil
}

// SetDefaultCompany делает компанию основной для клиента
func (s *CompanyService) SetDefaultCompany(ctx context.Context, clientID, companyID uuid.UUID) error {
	return s.repo.SetDefault(ctx, clientID, companyID)
}

// DetachCompany отвязывает компанию от клиента
func (s *CompanyService) DetachCompany(ctx context.Context, clientID, companyID uuid.UUID) error {
	return s.repo.DetachCompany(ctx, clientID, companyID)
}
