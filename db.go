package main

import (
	"database/sql"
	"os"
)

// ensureTable creates the necessary tables if they don't exist.
func ensureTable(db *sql.DB) error {
	// images table (kept for backwards compatibility)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS images (
        id BIGINT AUTO_INCREMENT PRIMARY KEY,
        url TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`); err != nil {
		return err
	}

	// products table for the shop
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS products (
        id BIGINT AUTO_INCREMENT PRIMARY KEY,
        title VARCHAR(255) NOT NULL,
        description TEXT,
        price DECIMAL(10,2) DEFAULT 0.00,
        image_url TEXT,
        category_id BIGINT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        INDEX idx_products_category (category_id)
    )`); err != nil {
		return err
	}

	// profile table for Linktree content (single row)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS profile (
        id TINYINT PRIMARY KEY,
        display_name VARCHAR(255) NOT NULL,
        username VARCHAR(255),
        bio TEXT,
        highlight TEXT,
        avatar_url TEXT,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
    )`); err != nil {
		return err
	}

	// seed default profile row if missing
	if _, err := db.Exec(`INSERT INTO profile (id, display_name, username, bio, highlight, avatar_url)
        SELECT 1, 'Mua Rẻ - Mặc Đẹp', '@lynvhu.passio.eco', 'Local curated closet • Giao nhanh trong 48h', 'Nhắn mình trên Instagram để chốt đơn nhé!', ''
        WHERE NOT EXISTS (SELECT 1 FROM profile WHERE id = 1)`); err != nil {
		return err
	}

	// categories table
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS categories (
        id BIGINT AUTO_INCREMENT PRIMARY KEY,
        name VARCHAR(255) NOT NULL UNIQUE
    )`); err != nil {
		return err
	}

	// socials table for profile social links (icons stored under static/img)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS socials (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		url TEXT NOT NULL,
		icon VARCHAR(255),
		ord INT DEFAULT 0
	)`); err != nil {
		return err
	}

	// seed default categories only in DEV_MODE. In production we avoid auto-creating categories
	// so that admin deletions are permanent and categories are managed explicitly.
	if os.Getenv("DEV_MODE") == "true" {
		if _, err := db.Exec(`INSERT INTO categories (name)
		SELECT * FROM (SELECT 'Quần áo' UNION SELECT 'Đầm' UNION SELECT 'Giày dép') AS defaults
		WHERE NOT EXISTS (SELECT 1 FROM categories)`); err != nil {
			return err
		}
	}

	// ensure category_id column exists (in case table was created earlier without)
	if _, err := db.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS category_id BIGINT NULL`); err == nil {
		_, _ = db.Exec(`ALTER TABLE products ADD INDEX IF NOT EXISTS idx_products_category (category_id)`)
	}

	// ensure external_url column exists for products (used for Shopee/external links)
	if _, err := db.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS external_url TEXT`); err != nil {
		return err
	}

	// ensure tag column exists for products to mark 'mychoice' vs 'shopee'
	if _, err := db.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS tag VARCHAR(16) DEFAULT 'mychoice'`); err != nil {
		return err
	}

	// ensure image_public_id column exists to allow deleting images from Cloudinary
	if _, err := db.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS image_public_id TEXT`); err != nil {
		return err
	}

	return nil
}
