package delivery

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	maxAvatarBytes  = 5 << 20
	avatarURLPrefix = "/uploads/avatars/"
)

type ProfileHandler struct {
	profile ProfileUsecase
}

func NewProfileHandler(profile ProfileUsecase) *ProfileHandler {
	return &ProfileHandler{profile: profile}
}

type cityResponse struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}

type profileResponse struct {
	ID            int32         `json:"id"`
	Email         string        `json:"email"`
	FirstName     string        `json:"first_name"`
	Phone         string        `json:"phone"`
	PhoneVerified bool          `json:"phone_verified"`
	City          *cityResponse `json:"city"`
	About         string        `json:"about"`
	AvatarURL     *string       `json:"avatar_url"`
	Rating        float64       `json:"rating"`
	ReviewsCount  int           `json:"reviews_count"`
	CreatedAt     time.Time     `json:"created_at"`
}

func newProfileResponse(p domain.Profile) profileResponse {
	resp := profileResponse{
		ID:            p.ID,
		Email:         p.Email,
		FirstName:     p.FirstName,
		Phone:         p.Phone,
		PhoneVerified: p.PhoneVerified,
		About:         p.About,
		Rating:        p.Rating,
		ReviewsCount:  p.ReviewsCount,
		CreatedAt:     p.CreatedAt,
	}
	if p.City != nil {
		resp.City = &cityResponse{ID: p.City.ID, Name: p.City.Name}
	}
	if p.Avatar != "" {
		url := avatarURL(p.Avatar)
		resp.AvatarURL = &url
	}
	return resp
}

func avatarURL(name string) string {
	return avatarURLPrefix + name
}

type updateProfileRequest struct {
	FirstName *string `json:"first_name"`
	Phone     *string `json:"phone"`
	Email     *string `json:"email"`
	CityID    *int32  `json:"city_id"`
	About     *string `json:"about"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	profile, err := h.profile.Profile(r.Context(), userIDFromContext(r.Context()))
	if err != nil {
		slog.ErrorContext(r.Context(), "get profile", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, newProfileResponse(profile))
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	profile, err := h.profile.UpdateProfile(r.Context(), userIDFromContext(r.Context()), domain.ProfileUpdate{
		FirstName: req.FirstName,
		Phone:     req.Phone,
		Email:     req.Email,
		CityID:    req.CityID,
		About:     req.About,
	})
	if err != nil {
		field := profileErrorField(err)
		if field == "" {
			slog.ErrorContext(r.Context(), "update profile", "err", err)
			writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
			return
		}

		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrEmailTaken) || errors.Is(err, domain.ErrPhoneTaken) {
			status = http.StatusConflict
		}
		writeFieldError(r.Context(), w, status, field, err.Error())
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, newProfileResponse(profile))
}

func profileErrorField(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidCity):
		return "city_id"
	case errors.Is(err, domain.ErrAboutTooLong):
		return "about"
	}
	return registerErrorField(err)
}

func (h *ProfileHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	cookie, _ := r.Cookie(sessionCookieName) // проверена в RequireAuth

	err := h.profile.ChangePassword(r.Context(), userIDFromContext(r.Context()), cookie.Value, req.CurrentPassword, req.NewPassword)
	switch {
	case err == nil:
		writeJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, domain.ErrWrongPassword):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "current_password", err.Error())
	case errors.Is(err, domain.ErrWeakPassword), errors.Is(err, domain.ErrPasswordTooLong):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "new_password", err.Error())
	default:
		slog.ErrorContext(r.Context(), "change password", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}

func (h *ProfileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+64<<10) // +64 КБ на заголовки multipart

	data, err := readFilePart(r)
	if err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge), errors.Is(err, errFileTooLarge):
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "file is too large")
		default:
			writeFieldError(r.Context(), w, http.StatusBadRequest, "file", "file is required")
		}
		return
	}

	name, err := h.profile.UploadAvatar(r.Context(), userIDFromContext(r.Context()), data)
	switch {
	case err == nil:
		writeJSON(r.Context(), w, http.StatusOK, map[string]string{"avatar_url": avatarURL(name)})
	case errors.Is(err, domain.ErrUnsupportedImage):
		writeError(r.Context(), w, http.StatusUnsupportedMediaType, err.Error())
	case errors.Is(err, domain.ErrInvalidImage):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "file", err.Error())
	default:
		slog.ErrorContext(r.Context(), "upload avatar", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}

var (
	errNoFile       = errors.New("no file part")
	errFileTooLarge = errors.New("file is too large")
)

func readFilePart(r *http.Request) ([]byte, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil, errNoFile
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() != "file" {
			continue
		}

		data, err := io.ReadAll(io.LimitReader(part, maxAvatarBytes+1))
		if err != nil {
			return nil, err
		}
		if len(data) > maxAvatarBytes {
			return nil, errFileTooLarge
		}
		if len(data) == 0 {
			return nil, errNoFile
		}
		return data, nil
	}
}
