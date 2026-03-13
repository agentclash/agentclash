package api

import "net/http"

type healthResponse struct {
	OK      bool   `json:"ok"`
	Service string `json:"service"`
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		OK:      true,
		Service: "api-server",
	})
}
