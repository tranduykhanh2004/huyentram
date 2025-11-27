package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// isAdmin checks the session cookie for a simple admin flag.
func isAdmin(r *http.Request) bool {
	c, err := r.Cookie("session")
	if err != nil {
		// no cookie, fallback to token header/query
	} else if c.Value == "admin" {
		return true
	}
	adminToken := os.Getenv("ADMIN_TOKEN")
	if adminToken == "" {
		return false
	}
	headerToken := r.Header.Get("X-Admin-Token")
	if headerToken != "" && headerToken == adminToken {
		return true
	}
	queryToken := r.URL.Query().Get("token")
	if queryToken != "" && queryToken == adminToken {
		return true
	}
	return false
}

// loginHandler expects JSON {"username","password"} and sets a session cookie for admin.
func loginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var cred struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cred); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if cred.Username == "admin" && cred.Password == "admin123" {
			http.SetCookie(w, &http.Cookie{
				Name:     "session",
				Value:    "admin",
				Path:     "/",
				HttpOnly: true,
				// In production set Secure: true and SameSite
			})
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

func logoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
		w.WriteHeader(http.StatusOK)
	}
}

// listProducts returns JSON list of products (public)
func listProducts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("listProducts called, method=%s, devMode=%t, remote=%s", r.Method, db == nil, r.RemoteAddr)
		// support dev-mode with in-memory store when db is nil
		if db == nil {
			out := DevGetProducts()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return
		}

		rows, err := db.Query(`SELECT p.id, p.title, p.description, p.price, p.image_url, IFNULL(p.category_id, 0), IFNULL(c.name,''), p.created_at
            FROM products p
            LEFT JOIN categories c ON c.id = p.category_id
            ORDER BY p.id DESC`)
		if err != nil {
			log.Println("listProducts db.Query error:", err)
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var out []Product
		for rows.Next() {
			var p Product
			var priceStr string
			var created interface{}
			var catNull sql.NullInt64
			if err := rows.Scan(&p.ID, &p.Title, &p.Description, &priceStr, &p.ImageURL, &catNull, &p.Category, &created); err != nil {
				log.Println("listProducts rows.Scan error:", err)
				http.Error(w, "db scan error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			// price comes as string from DECIMAL
			p.Price, _ = strconv.ParseFloat(priceStr, 64)
			// handle created_at which may be time.Time or []byte/string depending on driver
			switch v := created.(type) {
			case nil:
				p.CreatedAt = ""
			case string:
				p.CreatedAt = v
			case []byte:
				p.CreatedAt = string(v)
			case time.Time:
				p.CreatedAt = v.Format(time.RFC3339)
			default:
				p.CreatedAt = ""
			}
			if catNull.Valid {
				p.CategoryID = catNull.Int64
			} else {
				p.CategoryID = 0
			}
			out = append(out, p)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

// createProduct accepts multipart form with fields: title, description, price and file=file
// Only accessible to admin (cookie-based simple auth)
func createProduct(db *sql.DB, cloudURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAdmin(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(20 << 20); err != nil {
			http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
			return
		}

		title := r.FormValue("title")
		description := r.FormValue("description")
		priceStr := r.FormValue("price")
		categoryStr := r.FormValue("category_id")
		if title == "" {
			http.Error(w, "title required", http.StatusBadRequest)
			return
		}
		price, _ := strconv.ParseFloat(priceStr, 64)
		categoryID, _ := strconv.ParseInt(categoryStr, 10, 64)

		file, _, err := r.FormFile("file")
		var imageURL string

		// dev mode (no DB) -> store with placeholder image and in-memory store
		if db == nil {
			if categoryID != 0 {
				if _, ok := DevGetCategory(categoryID); !ok {
					http.Error(w, "category not found", http.StatusBadRequest)
					return
				}
			}
			if err == nil {
				// we won't upload to Cloudinary in dev; consume file to avoid leaks
				_ = file.Close()
			}
			if cloudURL == "" {
				imageURL = "https://via.placeholder.com/800x600.png?text=DEV+IMAGE"
			} else {
				imageURL = ""
			}
			id := DevAddProduct(title, description, price, imageURL, categoryID)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "image_url": imageURL})
			return
		}

		if categoryID != 0 {
			var name string
			if err := db.QueryRow("SELECT name FROM categories WHERE id = ?", categoryID).Scan(&name); err != nil {
				http.Error(w, "category not found", http.StatusBadRequest)
				return
			}
		}

		if err == nil {
			defer file.Close()
			// upload to Cloudinary
			cld, err := cloudinary.NewFromURL(cloudURL)
			if err != nil {
				log.Println("cloudinary init:", err)
				http.Error(w, "cloudinary init error", http.StatusInternalServerError)
				return
			}
			ctx := context.Background()
			uploadResult, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{})
			if err != nil {
				log.Println("upload error:", err)
				http.Error(w, "upload failed", http.StatusInternalServerError)
				return
			}
			imageURL = uploadResult.SecureURL
		} else {
			// no file provided is allowed; imageURL remains empty
			imageURL = ""
		}

		res, err := db.Exec("INSERT INTO products (title, description, price, image_url, category_id, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			title, description, FormatPrice(price), imageURL, sqlNull(categoryID), time.Now())
		if err != nil {
			log.Println("db insert error:", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "image_url": imageURL})
	}
}

// productItemHandler handles GET/PUT/DELETE for /api/products/{id}
func productItemHandler(db *sql.DB, cloudURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// path is /api/products/{id}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		idStr := parts[3]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// get single product
			if db == nil {
				// dev
				for _, p := range DevGetProducts() {
					if p.ID == id {
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(p)
						return
					}
				}
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			var p Product
			var priceStr string
			row := db.QueryRow(`SELECT p.id, p.title, p.description, p.price, p.image_url, IFNULL(p.category_id, 0), IFNULL(c.name,''), p.created_at
				FROM products p LEFT JOIN categories c ON c.id = p.category_id WHERE p.id = ?`, id)
			var created interface{}
			var catNull sql.NullInt64
			if err := row.Scan(&p.ID, &p.Title, &p.Description, &priceStr, &p.ImageURL, &catNull, &p.Category, &created); err != nil {
				log.Println("productItem GET row.Scan error:", err)
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			p.Price, _ = strconv.ParseFloat(priceStr, 64)
			if catNull.Valid {
				p.CategoryID = catNull.Int64
			} else {
				p.CategoryID = 0
			}

			switch v := created.(type) {
			case nil:
				p.CreatedAt = ""
			case string:
				p.CreatedAt = v
			case []byte:
				p.CreatedAt = string(v)
			case time.Time:
				p.CreatedAt = v.Format(time.RFC3339)
			default:
				p.CreatedAt = ""
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(p)
			return

		case http.MethodPut:
			// update product
			if !isAdmin(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if err := r.ParseMultipartForm(20 << 20); err != nil {
				http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
				return
			}
			// Determine which fields were provided in the multipart form so we can do partial updates
			mf := r.MultipartForm
			titleVals, hasTitle := mf.Value["title"]
			descVals, hasDesc := mf.Value["description"]
			priceVals, hasPrice := mf.Value["price"]
			catVals, hasCat := mf.Value["category_id"]
			// file presence
			file, _, ferr := r.FormFile("file")
			var imageURL string

			if db == nil {
				// Dev mode: load current product then apply only provided changes
				var cur *Product
				for _, p := range DevGetProducts() {
					if p.ID == id {
						cur = &p
						break
					}
				}
				if cur == nil {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				newTitle := cur.Title
				newDesc := cur.Description
				newPrice := cur.Price
				newCat := cur.CategoryID
				if hasTitle && len(titleVals) > 0 {
					newTitle = titleVals[0]
				}
				if hasDesc && len(descVals) > 0 {
					newDesc = descVals[0]
				}
				if hasPrice && len(priceVals) > 0 {
					if v, err := strconv.ParseFloat(priceVals[0], 64); err == nil {
						newPrice = v
					}
				}
				if hasCat && len(catVals) > 0 {
					if v, err := strconv.ParseInt(catVals[0], 10, 64); err == nil {
						newCat = v
					}
				}
				if ferr == nil {
					_ = file.Close()
					imageURL = "https://via.placeholder.com/800x600.png?text=DEV+IMAGE"
				}
				// validate category if provided
				if newCat != 0 {
					if _, ok := DevGetCategory(newCat); !ok {
						http.Error(w, "category not found", http.StatusBadRequest)
						return
					}
				}
				ok := DevUpdateProduct(id, newTitle, newDesc, newPrice, imageURL, newCat)
				if !ok {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			}

			// DB mode: build dynamic UPDATE using only provided fields
			// validate category if provided
			var catID int64
			if hasCat && len(catVals) > 0 {
				if v, err := strconv.ParseInt(catVals[0], 10, 64); err == nil {
					catID = v
				}
				if catID != 0 {
					var name string
					if err := db.QueryRow("SELECT name FROM categories WHERE id = ?", catID).Scan(&name); err != nil {
						http.Error(w, "category not found", http.StatusBadRequest)
						return
					}
				}
			}

			// handle file upload if present
			if ferr == nil {
				defer file.Close()
				cld, err := cloudinary.NewFromURL(cloudURL)
				if err != nil {
					log.Println("cloudinary init:", err)
					http.Error(w, "cloudinary init error", http.StatusInternalServerError)
					return
				}
				ctx := context.Background()
				uploadResult, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{})
				if err != nil {
					log.Println("upload error:", err)
					http.Error(w, "upload failed", http.StatusInternalServerError)
					return
				}
				imageURL = uploadResult.SecureURL
			}

			setCols := []string{}
			args := []interface{}{}
			if hasTitle && len(titleVals) > 0 {
				setCols = append(setCols, "title = ?")
				args = append(args, titleVals[0])
			}
			if hasDesc && len(descVals) > 0 {
				setCols = append(setCols, "description = ?")
				args = append(args, descVals[0])
			}
			if hasPrice && len(priceVals) > 0 {
				if v, err := strconv.ParseFloat(priceVals[0], 64); err == nil {
					setCols = append(setCols, "price = ?")
					args = append(args, FormatPrice(v))
				}
			}
			if ferr == nil && imageURL != "" {
				setCols = append(setCols, "image_url = ?")
				args = append(args, imageURL)
			}
			if hasCat {
				setCols = append(setCols, "category_id = ?")
				args = append(args, sqlNull(catID))
			}
			if len(setCols) == 0 {
				http.Error(w, "no fields to update", http.StatusBadRequest)
				return
			}
			// build query
			query := "UPDATE products SET " + strings.Join(setCols, ", ") + " WHERE id = ?"
			args = append(args, id)
			_, err := db.Exec(query, args...)
			if err != nil {
				log.Println("db update error:", err)
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return

		case http.MethodDelete:
			adminOk := isAdmin(r)
			log.Printf("product DELETE request id=%d remote=%s isAdmin=%t dbNil=%t", id, r.RemoteAddr, adminOk, db == nil)
			if !adminOk {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if db == nil {
				ok := DevDeleteProduct(id)
				if !ok {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			}
			res, err := db.Exec("DELETE FROM products WHERE id=?", id)
			if err != nil {
				log.Println("db delete error:", err)
				http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				log.Printf("product DELETE id=%d: not found (rows affected=0)", id)
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			log.Printf("product DELETE id=%d: deleted (rows affected>0)", id)
			w.WriteHeader(http.StatusOK)
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}
}

func categoriesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("categoriesHandler called method=%s remote=%s admin=%t dbNil=%t", r.Method, r.RemoteAddr, isAdmin(r), db == nil)
		switch r.Method {
		case http.MethodGet:
			var cats []Category
			if db == nil {
				cats = DevGetCategories()
			} else {
				rows, err := db.Query("SELECT id, name FROM categories ORDER BY name ASC")
				if err != nil {
					log.Println("categoriesHandler db.Query error:", err)
					http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				defer rows.Close()
				for rows.Next() {
					var c Category
					if err := rows.Scan(&c.ID, &c.Name); err != nil {
						http.Error(w, "scan error", http.StatusInternalServerError)
						return
					}
					cats = append(cats, c)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(cats)
			return

		case http.MethodPost:
			log.Printf("categoriesHandler POST from %s admin=%t", r.RemoteAddr, isAdmin(r))
			if !isAdmin(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
				return
			}
			payload.Name = strings.TrimSpace(payload.Name)
			if payload.Name == "" {
				http.Error(w, "name required", http.StatusBadRequest)
				return
			}
			if db == nil {
				c := DevAddCategory(payload.Name)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(c)
				return
			}
			res, err := db.Exec("INSERT INTO categories (name) VALUES (?)", payload.Name)
			if err != nil {
				log.Println("categories POST db.Exec error:", err)
				http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			id, _ := res.LastInsertId()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Category{ID: id, Name: payload.Name})
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}
}

func categoryItemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		log.Printf("categoryItemHandler called method=%s id=%d remote=%s admin=%t dbNil=%t", r.Method, id, r.RemoteAddr, isAdmin(r), db == nil)

		switch r.Method {
		case http.MethodPut:
			if !isAdmin(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			payload.Name = strings.TrimSpace(payload.Name)
			if payload.Name == "" {
				http.Error(w, "name required", http.StatusBadRequest)
				return
			}
			if db == nil {
				if !DevUpdateCategory(id, payload.Name) {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			}
			res, err := db.Exec("UPDATE categories SET name=? WHERE id=?", payload.Name, id)
			if err != nil {
				log.Println("category PUT db.Exec error:", err)
				http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			return

		case http.MethodDelete:
			if !isAdmin(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if db == nil {
				if !DevDeleteCategory(id) {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			}
			if _, err := db.Exec("UPDATE products SET category_id=NULL WHERE category_id=?", id); err != nil {
				log.Println("category DELETE update products error:", err)
				http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			res, err := db.Exec("DELETE FROM categories WHERE id=?", id)
			if err != nil {
				log.Println("category DELETE db.Exec error:", err)
				http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}
}

func sqlNull(id int64) interface{} {
	if id == 0 {
		return nil
	}
	return id
}

// profileHandler manages GET/PUT profile info for the Linktree layout.
func profileHandler(db *sql.DB, cloudURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			p, err := fetchProfile(db)
			if err != nil {
				http.Error(w, "profile not ready", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(p)
			return

		case http.MethodPut, http.MethodPost:
			if !isAdmin(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
				return
			}
			displayName := strings.TrimSpace(r.FormValue("display_name"))
			username := strings.TrimSpace(r.FormValue("username"))
			bio := strings.TrimSpace(r.FormValue("bio"))
			highlight := strings.TrimSpace(r.FormValue("highlight"))
			if displayName == "" {
				http.Error(w, "display_name is required", http.StatusBadRequest)
				return
			}
			current, err := fetchProfile(db)
			if err != nil {
				http.Error(w, "profile not ready", http.StatusInternalServerError)
				return
			}
			avatarURL := current.AvatarURL
			file, _, ferr := r.FormFile("avatar")
			if ferr == nil {
				if db == nil || cloudURL == "" {
					avatarURL = "https://via.placeholder.com/400.png?text=Avatar"
					_ = file.Close()
				} else {
					defer file.Close()
					cld, err := cloudinary.NewFromURL(cloudURL)
					if err != nil {
						log.Println("cloudinary init:", err)
						http.Error(w, "cloudinary init error", http.StatusInternalServerError)
						return
					}
					ctx := context.Background()
					uploadResult, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{})
					if err != nil {
						log.Println("upload error:", err)
						http.Error(w, "upload failed", http.StatusInternalServerError)
						return
					}
					avatarURL = uploadResult.SecureURL
				}
			}

			toSave := Profile{
				DisplayName: displayName,
				Username:    username,
				Bio:         bio,
				Highlight:   highlight,
				AvatarURL:   avatarURL,
			}
			if err := saveProfile(db, toSave); err != nil {
				log.Println("save profile:", err)
				http.Error(w, "failed to save profile", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(toSave)
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}
}

// adminDeleteProduct provides a POST JSON endpoint {"id":<number>} to delete a product.
func adminDeleteProduct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isAdmin(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var payload struct {
			ID int64 `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		id := payload.ID
		log.Printf("adminDeleteProduct called id=%d remote=%s dbNil=%t", id, r.RemoteAddr, db == nil)
		if db == nil {
			if !DevDeleteProduct(id) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		res, err := db.Exec("DELETE FROM products WHERE id=?", id)
		if err != nil {
			log.Println("adminDeleteProduct db.Exec error:", err)
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// adminDeleteCategory provides a POST JSON endpoint {"id":<number>} to delete a category.
func adminDeleteCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isAdmin(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var payload struct {
			ID int64 `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}
		id := payload.ID
		log.Printf("adminDeleteCategory called id=%d remote=%s dbNil=%t", id, r.RemoteAddr, db == nil)
		if db == nil {
			if !DevDeleteCategory(id) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if _, err := db.Exec("UPDATE products SET category_id=NULL WHERE category_id=?", id); err != nil {
			log.Println("adminDeleteCategory update products error:", err)
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		res, err := db.Exec("DELETE FROM categories WHERE id=?", id)
		if err != nil {
			log.Println("adminDeleteCategory db.Exec error:", err)
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
