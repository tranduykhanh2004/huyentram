package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

// startSelfPing periodically requests pingURL using the provided context.
// If pingURL is empty, it does nothing. interval is the duration between pings.
func startSelfPing(ctx context.Context, pingURL string, interval time.Duration) {
	if pingURL == "" {
		return
	}
	client := &http.Client{Timeout: 8 * time.Second}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		log.Printf("self-pinger enabled, pinging %s every %s", pingURL, interval)
		for {
			select {
			case <-ctx.Done():
				log.Println("self-pinger stopped")
				return
			case <-ticker.C:
				resp, err := client.Get(pingURL)
				if err != nil {
					log.Printf("self-ping error: %v", err)
					continue
				}
				resp.Body.Close()
				log.Printf("self-ping status: %s", resp.Status)
			}
		}
	}()
}

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	cloudURL := os.Getenv("CLOUDINARY_URL")
	devMode := false
	if v := os.Getenv("DEV_MODE"); v == "1" || strings.ToLower(v) == "true" {
		devMode = true
	}
	if !devMode {
		if dsn == "" || cloudURL == "" {
			log.Fatal("env MYSQL_DSN and CLOUDINARY_URL must be set (or set DEV_MODE=true to run without external services)")
		}
	}

	// If DSN requests tls=tidb, register a TLS config named "tidb".
	if strings.Contains(dsn, "tls=tidb") {
		// Try to load CA file from env TIDB_CA or default path used by TiDB Cloud docs
		caPath := os.Getenv("TIDB_CA")
		if caPath == "" {
			caPath = "/etc/ssl/certs/ca-certificates.crt"
		}
		pool := x509.NewCertPool()
		if b, err := os.ReadFile(caPath); err == nil {
			if ok := pool.AppendCertsFromPEM(b); ok {
				mysql.RegisterTLSConfig("tidb", &tls.Config{RootCAs: pool})
			} else {
				// fallback to InsecureSkipVerify if certs can't be parsed
				log.Printf("warning: could not parse CA file %s, falling back to InsecureSkipVerify", caPath)
				mysql.RegisterTLSConfig("tidb", &tls.Config{InsecureSkipVerify: true})
			}
		} else {
			log.Printf("warning: could not read CA file %s: %v, falling back to InsecureSkipVerify", caPath, err)
			mysql.RegisterTLSConfig("tidb", &tls.Config{InsecureSkipVerify: true})
		}
	}

	var db *sql.DB
	var err error
	if !devMode {
		// If DSN requests tls=tidb, register a TLS config named "tidb".
		if strings.Contains(dsn, "tls=tidb") {
			// Try to load CA file from env TIDB_CA or default path used by TiDB Cloud docs
			caPath := os.Getenv("TIDB_CA")
			if caPath == "" {
				caPath = "/etc/ssl/certs/ca-certificates.crt"
			}
			pool := x509.NewCertPool()
			if b, err := os.ReadFile(caPath); err == nil {
				if ok := pool.AppendCertsFromPEM(b); ok {
					mysql.RegisterTLSConfig("tidb", &tls.Config{RootCAs: pool})
				} else {
					log.Printf("warning: could not parse CA file %s, falling back to InsecureSkipVerify", caPath)
					mysql.RegisterTLSConfig("tidb", &tls.Config{InsecureSkipVerify: true})
				}
			} else {
				log.Printf("warning: could not read CA file %s: %v, falling back to InsecureSkipVerify", caPath, err)
				mysql.RegisterTLSConfig("tidb", &tls.Config{InsecureSkipVerify: true})
			}
		}

		db, err = sql.Open("mysql", dsn)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}
		defer db.Close()

		if err := db.Ping(); err != nil {
			log.Fatalf("ping db: %v", err)
		}

		if err := ensureTable(db); err != nil {
			log.Fatalf("ensure table: %v", err)
		}
	} else {
		log.Println("DEV_MODE=true: running without MySQL/Cloudinary (in-memory store, placeholder images)")
		// keep db == nil and cloudURL may be empty or set; handlers handle dev-mode when db==nil
		db = nil
		if cloudURL == "" {
			cloudURL = ""
		}
	}

	// Static assets and pages under /static
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Admin handler: allow login via secret link ?token=ADMIN_TOKEN or existing session cookie
	http.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		// if session cookie present, serve admin
		if isAdmin(r) {
			http.ServeFile(w, r, "./static/admin.html")
			return
		}
		// otherwise check token
		token := r.URL.Query().Get("token")
		if token != "" && token == os.Getenv("ADMIN_TOKEN") {
			// set admin cookie and serve
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "admin", Path: "/", HttpOnly: true})
			http.ServeFile(w, r, "./static/admin.html")
			return
		}
		// forbid direct access
		http.Error(w, "forbidden", http.StatusForbidden)
	})

	// API endpoints
	http.HandleFunc("/api/login", loginHandler())
	http.HandleFunc("/api/logout", logoutHandler())
	http.HandleFunc("/api/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listProducts(db)(w, r)
		case http.MethodPost:
			createProduct(db, cloudURL)(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// product item endpoints (GET/PUT/DELETE)
	http.HandleFunc("/api/products/", productItemHandler(db, cloudURL))
	// categories endpoints
	http.HandleFunc("/api/categories", categoriesHandler(db))
	http.HandleFunc("/api/categories/", categoryItemHandler(db))
	// socials endpoints and static images list
	http.HandleFunc("/api/socials", socialsHandler(db))
	http.HandleFunc("/api/socials/", socialItemHandler(db))
	http.HandleFunc("/api/static-imgs", staticImagesHandler)
	// simple ping endpoint used by self-pinger; protected by SELF_PING_TOKEN if set
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		expected := os.Getenv("SELF_PING_TOKEN")
		if expected != "" {
			got := r.URL.Query().Get("token")
			if got == "" || got != expected {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		fmt.Fprintln(w, "Pong")
	})
	// admin convenience endpoints for delete operations (POST JSON {id})
	http.HandleFunc("/api/admin/delete-product", adminDeleteProduct(db))
	http.HandleFunc("/api/admin/delete-category", adminDeleteCategory(db))
	// profile info endpoint
	http.HandleFunc("/api/profile", profileHandler(db, cloudURL))

	// Serve root files (index.html and admin.html live under ./static)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Let static handler serve files; default to index.html
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "./static/index.html")
			return
		}
		// For other top-level files, try static
		http.ServeFile(w, r, "./static"+r.URL.Path)
	})

	// Self-ping configuration: read URL and optional interval (minutes)
	pingURL := os.Getenv("SELF_PING_URL")
	if pingURL == "" {
		// fallback for older name
		pingURL = os.Getenv("RENDER_PING_URL")
	}
	intervalMin := 14
	if v := os.Getenv("SELF_PING_INTERVAL_MIN"); v != "" {
		if iv, err := strconv.Atoi(v); err == nil && iv > 0 {
			intervalMin = iv
		}
	}

	// create a cancellable context that listens for SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// start the self-pinger (no-op if pingURL is empty)
	startSelfPing(ctx, pingURL, time.Duration(intervalMin)*time.Minute)

	srv := &http.Server{Addr: ":8000"}
	go func() {
		log.Println("server listening on :8000")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// wait for interrupt
	<-ctx.Done()
	log.Println("shutdown signal received, shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	log.Println("server gracefully stopped")
}
