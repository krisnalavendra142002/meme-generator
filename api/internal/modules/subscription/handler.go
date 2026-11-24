package subscription

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/nyxsr/meme-portal/api/internal/middleware"
	"github.com/nyxsr/meme-portal/api/internal/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.service.ListPlans(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalServer, "gagal memuat paket")
		return
	}
	response.Success(w, http.StatusOK, CodePlansListed, plans, "daftar paket langganan")
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	status, err := h.service.Status(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalServer, "gagal memuat status langganan")
		return
	}
	response.Success(w, http.StatusOK, CodeStatusFetched, status, "status langganan")
}

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	var dto CheckoutDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil || dto.PlanCode == "" {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequestBody, "planCode wajib diisi")
		return
	}
	// Idempotency-Key wajib: satu maksud checkout = satu key. Klien yang
	// mengirim key sama (retry/double-click) mendapat checkout yang sama.
	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey == "" {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequestBody,
			"header Idempotency-Key wajib diisi")
		return
	}
	res, err := h.service.Checkout(r.Context(), userID, dto, idemKey)
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, CodeCheckoutCreated, res, "checkout dibuat")
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	reference := r.PathValue("reference")
	if reference == "" {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequestBody, "reference wajib diisi")
		return
	}
	sub, pay, err := h.service.Confirm(r.Context(), reference)
	if err != nil {
		writeError(w, err)
		return
	}
	response.Success(w, http.StatusOK, CodePaymentConfirmed, map[string]any{
		"subscription": sub,
		"payment":      pay,
	}, "pembayaran terkonfirmasi")
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPlanNotFound):
		response.Error(w, http.StatusNotFound, response.CodePlanNotFound, err.Error())
	case errors.Is(err, ErrAlreadySubscribed):
		response.Error(w, http.StatusConflict, response.CodeAlreadySubscribed, err.Error())
	case errors.Is(err, ErrPaymentFailed):
		response.Error(w, http.StatusPaymentRequired, response.CodePaymentFailed, err.Error())
	case errors.Is(err, ErrNotSubscribed):
		response.Error(w, http.StatusForbidden, response.CodeNotSubscribed, err.Error())
	default:
		// Log error asli agar error internal bisa didiagnosis dari log.
		slog.Error("subscription handler: internal error", "err", err)
		response.Error(w, http.StatusInternalServerError, response.CodeInternalServer, "internal server error")
	}
}
