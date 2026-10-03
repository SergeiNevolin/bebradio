package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bebradio/backend-go/internal/usecase"
)

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	if !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		s.writeError(w, 400, "Invalid email")
		return
	}
	if len(req.Username) < 2 || len(req.Username) > 30 {
		s.writeError(w, 400, "Username must be 2-30 characters")
		return
	}

	user, token, err := s.auth.Register(req.Email, req.Username, req.Password)
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("register failed", "error", err, "email", req.Email)
		s.writeError(w, 500, "Internal server error")
		return
	}

	s.issueRefreshCookie(w, r, user.ID)
	s.writeJSON(w, 200, map[string]any{
		"token": token,
		"user":  user.ProfileWithEmail(),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	user, token, err := s.auth.Login(req.Email, req.Password)
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("login failed", "error", err, "email", req.Email)
		s.writeError(w, 500, "Internal server error")
		return
	}

	s.issueRefreshCookie(w, r, user.ID)
	s.writeJSON(w, 200, map[string]any{
		"token": token,
		"user":  user.ProfileWithEmail(),
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}

	user, err := s.auth.GetUserByID(userID)
	if err != nil {
		s.writeError(w, 401, "User not found")
		return
	}

	s.writeJSON(w, 200, map[string]any{"user": user.ProfileWithEmail()})
}

// ── refresh-сессия: httpOnly-кука + ротация через Redis ─────────────────────

const refreshCookieName = "bebradio_refresh"

func (s *Server) refreshCookieMaxAge() int {
	days := s.config.JWTRefreshDays
	if days <= 0 {
		days = 30
	}
	return days * 24 * 3600
}

// issueRefreshCookie — best-effort: при сбое (Redis и т.п.) логин не валяем,
// просто не будет refresh (пользователь получит короткую сессию).
func (s *Server) issueRefreshCookie(w http.ResponseWriter, r *http.Request, userID string) {
	if s.refresh == nil {
		return
	}
	token, err := s.auth.CreateRefreshToken(userID)
	if err != nil {
		s.log.Error("create refresh token failed", "error", err)
		return
	}
	_, jti, err := s.auth.DecodeRefreshToken(token)
	if err != nil {
		s.log.Error("decode own refresh token failed", "error", err)
		return
	}
	if err := s.refresh.Save(r.Context(), jti, userID); err != nil {
		s.log.Error("save refresh token failed", "error", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/api/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.config.AuthCookieSecure,
		MaxAge:   s.refreshCookieMaxAge(),
	})
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.config.AuthCookieSecure,
		MaxAge:   -1,
	})
}

// handleRefresh — POST /api/auth/refresh: валидирует куку, ротирует refresh,
// отдаёт новый access. Любой отказ — 401 (фронт разлогинивает).
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if s.refresh == nil {
		s.writeError(w, 503, "Refresh unavailable")
		return
	}
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	sub, jti, err := s.auth.DecodeRefreshToken(cookie.Value)
	if err != nil {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	alive, err := s.refresh.Alive(r.Context(), jti)
	if err != nil {
		s.log.Error("refresh store check failed", "error", err)
		s.writeError(w, 503, "Refresh unavailable")
		return
	}
	if !alive {
		// отозван, уже использован или истёк по TTL
		s.writeError(w, 401, "Not authenticated")
		return
	}

	access, err := s.auth.CreateToken(sub)
	if err != nil {
		s.log.Error("create access token failed", "error", err)
		s.writeError(w, 500, "Internal server error")
		return
	}

	// ротация: сначала сохраняем новый, только потом отзываем старый —
	// иначе сбой сохранения убьёт единственную живую сессию
	newToken, err := s.auth.CreateRefreshToken(sub)
	if err != nil {
		s.log.Error("rotate refresh token failed", "error", err)
		s.writeError(w, 500, "Internal server error")
		return
	}
	_, newJTI, err := s.auth.DecodeRefreshToken(newToken)
	if err != nil {
		s.log.Error("decode rotated refresh token failed", "error", err)
		s.writeError(w, 500, "Internal server error")
		return
	}
	if err := s.refresh.Save(r.Context(), newJTI, sub); err != nil {
		s.log.Error("save rotated refresh token failed", "error", err)
		s.writeError(w, 503, "Refresh unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    newToken,
		Path:     "/api/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.config.AuthCookieSecure,
		MaxAge:   s.refreshCookieMaxAge(),
	})
	if err := s.refresh.Revoke(r.Context(), jti); err != nil {
		s.log.Error("revoke old refresh token failed", "error", err)
	}

	s.writeJSON(w, 200, map[string]any{"token": access})
}

// handleLogout — POST /api/auth/logout: отзыв refresh + очистка куки.
// Работает и без куки (идемпотентен).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookieName); err == nil && s.refresh != nil {
		if _, jti, err := s.auth.DecodeRefreshToken(cookie.Value); err == nil {
			if err := s.refresh.Revoke(r.Context(), jti); err != nil {
				s.log.Error("logout revoke failed", "error", err)
			}
		}
	}
	s.clearRefreshCookie(w)
	s.writeJSON(w, 200, map[string]any{"ok": true})
}
