package main

import (
	"database/sql"
	"fmt"
)

// fetchProfile returns the single profile row (or dev profile when db==nil).
func fetchProfile(db *sql.DB) (Profile, error) {
	if db == nil {
		return DevGetProfile(), nil
	}
	var p Profile
	row := db.QueryRow("SELECT display_name, username, bio, highlight, avatar_url FROM profile WHERE id = 1")
	if err := row.Scan(&p.DisplayName, &p.Username, &p.Bio, &p.Highlight, &p.AvatarURL); err != nil {
		return Profile{}, fmt.Errorf("scan profile: %w", err)
	}
	return p, nil
}

// saveProfile updates the profile row or in-memory profile in dev mode.
func saveProfile(db *sql.DB, p Profile) error {
	if db == nil {
		DevUpdateProfile(p)
		return nil
	}
	_, err := db.Exec(`UPDATE profile SET display_name=?, username=?, bio=?, highlight=?, avatar_url=? WHERE id = 1`,
		p.DisplayName, p.Username, p.Bio, p.Highlight, p.AvatarURL)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}
