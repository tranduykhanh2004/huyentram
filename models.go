package main

// Product represents a product in the shop.
type Product struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Price         float64 `json:"price"`
	ImageURL      string  `json:"image_url"`
	ImagePublicID string  `json:"image_public_id"`
	ExternalURL   string  `json:"external_url"`
	Tag           string  `json:"tag"`
	CategoryID    int64   `json:"category_id"`
	Category      string  `json:"category"`
	CreatedAt     string  `json:"created_at"`
}

// Profile represents public store/profile info for the Linktree-style page.
type Profile struct {
	DisplayName string   `json:"display_name"`
	Username    string   `json:"username"`
	Bio         string   `json:"bio"`
	Highlight   string   `json:"highlight"`
	AvatarURL   string   `json:"avatar_url"`
	Socials     []Social `json:"socials,omitempty"`
}

// Social represents a social network link shown on the profile (ordered).
type Social struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Icon string `json:"icon"` // filename under static/img
	Ord  int    `json:"ord"`
}

// Category represents a product category/tag.
type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
