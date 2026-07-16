package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

const (
	// Default hyperlight-unikraft guest mount path for --hyperlight-mount.
	resPath = "/mnt/res.txt"
	reqPath = "/mnt/req.txt"
)

func rootHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(resPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("read %s: %v", resPath, err), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(reqPath, body, 0o644); err != nil {
			http.Error(w, fmt.Sprintf("write %s: %v", reqPath, err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)

	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
