package main

import (
	"log"
	"net/http"

	"github.com/EOEboh/mb-project-06-image-captioner/handlers"
)

func main() {
	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Routes
	//   GET  /        → upload page
	//   POST /caption → receive image, return AI-generated caption as HTML fragment
	mux.HandleFunc("/", handlers.Index)
	mux.HandleFunc("POST /caption", handlers.Caption)

	log.Println("Image Captioner running => http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
