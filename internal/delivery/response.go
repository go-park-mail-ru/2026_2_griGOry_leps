package delivery

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeJSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.ErrorContext(ctx, "write json response", "err", err)
	}
}

func writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSON(ctx, w, status, map[string]string{"error": message})
}

func writeFieldError(ctx context.Context, w http.ResponseWriter, status int, field, message string) {
	writeJSON(ctx, w, status, map[string]string{"error": message, "field": field})
}
