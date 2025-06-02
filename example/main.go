package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/aarol/reload"
)

func main() {
	isDevelopment := flag.Bool("dev", true, "Development mode")
	flag.Parse()

	hopts := slog.HandlerOptions{Level: slog.LevelInfo}
	if *isDevelopment {
		hopts.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &hopts))

	templateCache := parseTemplates()

	// serve any static files like normal
	http.Handle("/static/", http.FileServer(http.Dir("ui/")))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// serve a template file with dynamic data
		data := map[string]any{
			"Timestamp": time.Now().Format("Monday, 02-Jan-06 15:04:05 MST"),
		}
		err := templateCache.ExecuteTemplate(w, "index.html", data)
		if err != nil {
			logger.ErrorContext(r.Context(), "execute template", "err", err)
		}
	})

	// handler can be anything that implements http.Handler,
	// like chi.Router, echo.Echo or gin.Engine
	var handler http.Handler = http.DefaultServeMux

	if *isDevelopment {
		// Call `New()` with a list of directories to recursively watch
		reload := reload.New("ui/")

		reload.OnReload = func() {
			templateCache = parseTemplates()
		}

		reload.Logger = logger.WithGroup("reload")

		handler = reload.Handle(handler)
	} else {
		logger.Error("running in production mode")
	}

	addr := "localhost:3001"

	logger.Info("Server running", "addr", fmt.Sprintf("http://%s", addr))

	http.ListenAndServe(addr, handler)
}

func parseTemplates() *template.Template {
	return template.Must(template.ParseGlob("ui/*.html"))
}
