package main

import (
	"fmt"
	"log"
	"net/http"

	"authserver/auth"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		token, err := auth.Login(username, password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		fmt.Fprintf(w, `{"token":"%s"}`, token)
	})

	mux.HandleFunc("/protected", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		user, err := auth.ValidateToken(token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		fmt.Fprintf(w, `{"message":"hello, %s"}`, user)
	})

	log.Println("server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
