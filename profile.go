package main

import (
	"database/sql"
	"fmt"
)

// fetchProfile returns the single profile row (or dev profile when db==nil).
func fetchProfile(db *sql.DB) (Profile, error) {
	if db == nil {
		p := DevGetProfile()
		// attach dev socials
		p.Socials = DevGetSocials()
		return p, nil
	}
	var p Profile
	row := db.QueryRow("SELECT display_name, username, bio, highlight, avatar_url FROM profile WHERE id = 1")
	if err := row.Scan(&p.DisplayName, &p.Username, &p.Bio, &p.Highlight, &p.AvatarURL); err != nil {
		return Profile{}, fmt.Errorf("scan profile: %w", err)
	}
	// load socials
	rows, err := db.Query("SELECT id, name, url, IFNULL(icon,''), ord FROM socials ORDER BY ord ASC, id ASC")
	if err == nil {
		defer rows.Close()
		var socs []Social
		for rows.Next() {
			var s Social
			if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.Icon, &s.Ord); err == nil {
				socs = append(socs, s)
			}
		}
		p.Socials = socs
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
