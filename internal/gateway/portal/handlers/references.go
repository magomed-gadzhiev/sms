package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ReferencesHandlers returns static reference data (operators, countries) for filter dropdowns.
type ReferencesHandlers struct {
	db *pgxpool.Pool
}

// NewReferencesHandlers creates a new ReferencesHandlers.
func NewReferencesHandlers(db *pgxpool.Pool) *ReferencesHandlers {
	return &ReferencesHandlers{db: db}
}

type operatorItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type countryItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	ISOCode string `json:"iso_code"`
}

// ListOperators handles GET /portal/v1/references/operators
func (h *ReferencesHandlers) ListOperators(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, name, code
		FROM operators
		WHERE active = true
		ORDER BY name
	`)
	if err != nil {
		log.Error().Err(err).Msg("references: failed to list operators")
		respondError(w, shared.ErrInternalServer("Ошибка получения операторов"))
		return
	}
	defer rows.Close()

	operators := []operatorItem{}
	for rows.Next() {
		var op operatorItem
		if err := rows.Scan(&op.ID, &op.Name, &op.Code); err != nil {
			log.Warn().Err(err).Msg("references: skipping malformed operator row")
			continue
		}
		operators = append(operators, op)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка итерации операторов"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"operators": operators})
}

// ListCountries handles GET /portal/v1/references/countries
func (h *ReferencesHandlers) ListCountries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, name, iso_code
		FROM countries
		ORDER BY name
	`)
	if err != nil {
		log.Error().Err(err).Msg("references: failed to list countries")
		respondError(w, shared.ErrInternalServer("Ошибка получения стран"))
		return
	}
	defer rows.Close()

	countries := []countryItem{}
	for rows.Next() {
		var c countryItem
		if err := rows.Scan(&c.ID, &c.Name, &c.ISOCode); err != nil {
			log.Warn().Err(err).Msg("references: skipping malformed country row")
			continue
		}
		countries = append(countries, c)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("Ошибка итерации стран"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"countries": countries})
}
