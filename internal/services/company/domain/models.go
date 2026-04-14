package domain

import (
	"errors"
	"strconv"
	"time"
	"unicode"

	"github.com/google/uuid"
)

var (
	ErrCompanyNotFound       = errors.New("company not found")
	ErrInvalidINN            = errors.New("invalid INN checksum or length")
	ErrCompanyNameRequired   = errors.New("company name is required")
	ErrCompanyHasSenderNames = errors.New("cannot detach company with active sender names")
	ErrCannotDetachDefault   = errors.New("cannot detach default company")
	ErrAlreadyDefault        = errors.New("company is already the default")
	ErrNotAttached           = errors.New("company is not attached to this client")
)

// Company — доменная модель компании
type Company struct {
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
	IsOffer         bool
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ValidateINN проверяет контрольную сумму ИНН (10 или 12 цифр).
// Пустая строка допустима (для компании «Оферта»).
func ValidateINN(inn string) bool {
	if inn == "" {
		return true
	}
	for _, r := range inn {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	switch len(inn) {
	case 10:
		coeffs := []int{2, 4, 10, 3, 5, 9, 4, 6, 8}
		sum := 0
		for i, c := range coeffs {
			d, _ := strconv.Atoi(string(inn[i]))
			sum += c * d
		}
		check := sum % 11 % 10
		last, _ := strconv.Atoi(string(inn[9]))
		return check == last
	case 12:
		c1 := []int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
		c2 := []int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
		s1 := 0
		for i, c := range c1 {
			d, _ := strconv.Atoi(string(inn[i]))
			s1 += c * d
		}
		n11, _ := strconv.Atoi(string(inn[10]))
		if s1%11%10 != n11 {
			return false
		}
		s2 := 0
		for i, c := range c2 {
			d, _ := strconv.Atoi(string(inn[i]))
			s2 += c * d
		}
		n12, _ := strconv.Atoi(string(inn[11]))
		return s2%11%10 == n12
	default:
		return false
	}
}
