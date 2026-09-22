package main

import (
	"log"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/config"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/handler"
)

func main() {
	cfg := config.Load()

	router := handler.NewRouter(cfg.FrontendOrigin)

	log.Printf("server starting on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}