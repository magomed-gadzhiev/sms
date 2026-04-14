package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	companyv1 "github.com/smpp-server/smpp-server/api/proto/companyv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type CompanyHandlers struct {
	client companyv1.CompanyServiceClient
}

func NewCompanyHandlers(client companyv1.CompanyServiceClient) *CompanyHandlers {
	return &CompanyHandlers{client: client}
}

type companyRequest struct {
	INN             string `json:"inn"`
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	KPP             string `json:"kpp"`
	OGRN            string `json:"ogrn"`
	LegalAddress    string `json:"legal_address"`
	ActualAddress   string `json:"actual_address"`
	CEOName         string `json:"ceo_name"`
	CEOTitle        string `json:"ceo_title"`
	ActingBasis     string `json:"acting_basis"`
	BankName        string `json:"bank_name"`
	BankBIK         string `json:"bank_bik"`
	BankCorrAccount string `json:"bank_corr_account"`
	BankAccount     string `json:"bank_account"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
}

func (h *CompanyHandlers) ListCompanies(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	resp, err := h.client.ListClientCompanies(r.Context(), &companyv1.ListClientCompaniesRequest{
		ClientId: clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"companies": resp.Companies,
	})
}

func (h *CompanyHandlers) CreateCompany(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var req companyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.client.CreateCompany(r.Context(), &companyv1.CreateCompanyRequest{
		ClientId: clientID.String(), Inn: req.INN, Name: req.Name,
		FullName: req.FullName, Kpp: req.KPP, Ogrn: req.OGRN,
		LegalAddress: req.LegalAddress, ActualAddress: req.ActualAddress,
		CeoName: req.CEOName, CeoTitle: req.CEOTitle, ActingBasis: req.ActingBasis,
		BankName: req.BankName, BankBik: req.BankBIK,
		BankCorrAccount: req.BankCorrAccount, BankAccount: req.BankAccount,
		Email: req.Email, Phone: req.Phone,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp.Company)
}

func (h *CompanyHandlers) GetCompany(w http.ResponseWriter, r *http.Request) {
	companyID := mux.Vars(r)["id"]
	if _, err := uuid.Parse(companyID); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный ID компании"))
		return
	}
	resp, err := h.client.GetCompany(r.Context(), &companyv1.GetCompanyRequest{CompanyId: companyID})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp.Company)
}

func (h *CompanyHandlers) UpdateCompany(w http.ResponseWriter, r *http.Request) {
	companyID := mux.Vars(r)["id"]
	if _, err := uuid.Parse(companyID); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный ID компании"))
		return
	}
	var req companyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.client.UpdateCompany(r.Context(), &companyv1.UpdateCompanyRequest{
		CompanyId: companyID, Inn: req.INN, Name: req.Name,
		FullName: req.FullName, Kpp: req.KPP, Ogrn: req.OGRN,
		LegalAddress: req.LegalAddress, ActualAddress: req.ActualAddress,
		CeoName: req.CEOName, CeoTitle: req.CEOTitle, ActingBasis: req.ActingBasis,
		BankName: req.BankName, BankBik: req.BankBIK,
		BankCorrAccount: req.BankCorrAccount, BankAccount: req.BankAccount,
		Email: req.Email, Phone: req.Phone,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp.Company)
}

func (h *CompanyHandlers) SetDefaultCompany(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	companyID := mux.Vars(r)["id"]
	_, err := h.client.SetDefaultCompany(r.Context(), &companyv1.SetDefaultCompanyRequest{
		ClientId:  clientID.String(),
		CompanyId: companyID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CompanyHandlers) DetachCompany(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	companyID := mux.Vars(r)["id"]
	_, err := h.client.DetachCompany(r.Context(), &companyv1.DetachCompanyRequest{
		ClientId:  clientID.String(),
		CompanyId: companyID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
