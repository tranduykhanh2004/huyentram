package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	mysql "github.com/go-sql-driver/mysql"
)

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

	log.Println("server listening on :8000")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatalf("server: %v", err)
	}
}
