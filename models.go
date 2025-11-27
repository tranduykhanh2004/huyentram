package main

// Product represents a product in the shop.
type Product struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url"`
	CategoryID  int64   `json:"category_id"`
	Category    string  `json:"category"`
	CreatedAt   string  `json:"created_at"`
}

// Profile represents public store/profile info for the Linktree-style page.
type Profile struct {
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
	Bio         string `json:"bio"`
	Highlight   string `json:"highlight"`
	AvatarURL   string `json:"avatar_url"`
}

// Category represents a product category/tag.
type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
