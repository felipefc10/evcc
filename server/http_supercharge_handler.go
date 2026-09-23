package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/evcc-io/evcc/core/supercharge"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const superchargeBodyLimit = 1 << 20

// RegisterSuperchargeHandlers connects the whole-house load management api.
// Changing the installation or its tuning needs the admin login, like evcc's own configuration.
func (s *HTTPd) RegisterSuperchargeHandlers(m *supercharge.Manager, ensureAuth mux.MiddlewareFunc) {
	if m == nil {
		return
	}

	router := s.Server.Handler.(*mux.Router)
	api := router.PathPrefix("/api/supercharging").Subrouter()
	api.Use(jsonHandler)
	api.Use(handlers.CompressHandler)

	routes := map[string]route{
		"state":       {"GET", "", superchargeStateHandler(m)},
		"config":      {"GET", "/config", superchargeConfigHandler(m)},
		"setconfig":   {"POST", "/config", ensureAuth(superchargeUpdateConfigHandler(m)).ServeHTTP},
		"supercharge": {"POST", "/loadpoints/{id:[0-9]+}/supercharge", superchargeLoadpointHandler(m)},
		"bursts":      {"GET", "/bursts", superchargeBurstsHandler(m)},
		"diagnostics": {"GET", "/diagnostics", superchargeDiagnosticsHandler(m)},
		"selftest":    {"POST", "/selftest", superchargeSelftestHandler(m)},
		"import":      {"POST", "/import", ensureAuth(superchargeImportHandler(m)).ServeHTTP},
	}

	for _, r := range routes {
		api.Methods(r.Methods()...).Path(r.Pattern).Handler(r.HandlerFunc)
	}
}

func superchargeStateHandler(m *supercharge.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonWrite(w, m.State())
	}
}

func superchargeConfigHandler(m *supercharge.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonWrite(w, m.Config())
	}
}

func superchargeUpdateConfigHandler(m *supercharge.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, superchargeBodyLimit))
		if err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		res, err := m.UpdateConfig(body)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		jsonWrite(w, res)
	}
}

func superchargeLoadpointHandler(m *supercharge.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil || id < 1 {
			jsonError(w, http.StatusBadRequest, errors.New("invalid loadpoint"))
			return
		}

		var req struct {
			On    bool   `json:"on"`
			Until string `json:"until"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, superchargeBodyLimit)).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}

		var until int64
		if req.On {
			if until, err = supercharge.ResolveUntil(req.Until, time.Now()); err != nil {
				jsonError(w, http.StatusBadRequest, err)
				return
			}
		}
		if err := m.SetSupercharge(id-1, req.On, until); err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		jsonWrite(w, m.State())
	}
}

func superchargeBurstsHandler(m *supercharge.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonWrite(w, m.Bursts())
	}
}

func superchargeDiagnosticsHandler(m *supercharge.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="supercharging-diagnostics.json"`)
		jsonWrite(w, m.Diagnostics())
	}
}

func superchargeSelftestHandler(m *supercharge.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonWrite(w, m.Selftest(r.Context()))
	}
}

func superchargeImportHandler(m *supercharge.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Settings map[string]any    `json:"settings"`
			Learned  json.RawMessage   `json:"learned"`
			Names    map[string]string `json:"names"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, superchargeBodyLimit)).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		n, err := m.ImportAddon(req.Settings, req.Learned, req.Names)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		jsonWrite(w, map[string]any{"learnedKeys": n, "config": m.Config()})
	}
}
