package main

import (
	"strconv"
	"sync"
	"time"
)

var (
	devMu         sync.Mutex
	devProducts   []Product
	devNextID     int64 = 1
	devCategories       = []Category{
		{ID: 1, Name: "Quần áo"},
		{ID: 2, Name: "Đầm"},
		{ID: 3, Name: "Giày dép"},
	}
	devNextCatID int64 = 4

	devProfile = Profile{
		DisplayName: "Mua Rẻ - Mặc Đẹp",
		Username:    "@lynvhu.passio.eco",
		Bio:         "Local curated closet • Giao nhanh trong 48h",
		Highlight:   "Nhắn mình trên Instagram để chốt đơn nhé!",
		AvatarURL:   "https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80",
	}

	devSocials = []Social{
		{ID: 1, Name: "Instagram", URL: "https://www.instagram.com/lynvhu.passio.eco", Icon: "instagram.png", Ord: 1},
		{ID: 2, Name: "Facebook", URL: "https://www.facebook.com/", Icon: "facebook.png", Ord: 2},
	}
	devNextSocialID int64 = 3
)

// DevGetProducts returns a copy of in-memory products for dev mode.
func DevGetProducts() []Product {
	devMu.Lock()
	defer devMu.Unlock()
	cp := make([]Product, len(devProducts))
	copy(cp, devProducts)
	return cp
}

// DevAddProduct adds a product to the in-memory store and returns the new id.
func DevAddProduct(title, description string, price float64, imageURL string, categoryID int64, externalURL string, tag string) int64 {
	devMu.Lock()
	defer devMu.Unlock()
	id := devNextID
	devNextID++
	categoryName := ""
	for _, c := range devCategories {
		if c.ID == categoryID {
			categoryName = c.Name
			break
		}
	}
	// default tag to mychoice when dev add (external_url may indicate shopee but admin should set tag)
	if tag == "" {
		tag = "mychoice"
	}
	p := Product{ID: id, Title: title, Description: description, Price: price, ImageURL: imageURL, ExternalURL: externalURL, Tag: tag, CategoryID: categoryID, Category: categoryName, CreatedAt: time.Now().Format(time.RFC3339)}
	devProducts = append([]Product{p}, devProducts...)
	return id
}

// DevDeleteProduct removes a product by id from the in-memory store.
func DevDeleteProduct(id int64) bool {
	devMu.Lock()
	defer devMu.Unlock()
	for i, p := range devProducts {
		if p.ID == id {
			devProducts = append(devProducts[:i], devProducts[i+1:]...)
			return true
		}
	}
	return false
}

// DevUpdateProduct updates a product in-memory. Returns true if found.
func DevUpdateProduct(id int64, title, description string, price float64, imageURL string, categoryID int64, externalURL string, tag string) bool {
	devMu.Lock()
	defer devMu.Unlock()
	for i, p := range devProducts {
		if p.ID == id {
			devProducts[i].Title = title
			devProducts[i].Description = description
			devProducts[i].Price = price
			devProducts[i].CategoryID = categoryID
			devProducts[i].Category = ""
			for _, c := range devCategories {
				if c.ID == categoryID {
					devProducts[i].Category = c.Name
					break
				}
			}
			if imageURL != "" {
				devProducts[i].ImageURL = imageURL
			}
			// update external URL if provided (empty string clears it)
			devProducts[i].ExternalURL = externalURL
			if tag != "" {
				devProducts[i].Tag = tag
			}
			devProducts[i].CreatedAt = time.Now().Format(time.RFC3339)
			return true
		}
	}
	return false
}

// helper to format price for DB compatibility if needed
func FormatPrice(p float64) string {
	return strconv.FormatFloat(p, 'f', 2, 64)
}

// DevGetProfile returns current in-memory profile.
func DevGetProfile() Profile {
	devMu.Lock()
	defer devMu.Unlock()
	return devProfile
}

// DevGetSocials returns copy of in-memory socials.
func DevGetSocials() []Social {
	devMu.Lock()
	defer devMu.Unlock()
	out := make([]Social, len(devSocials))
	copy(out, devSocials)
	return out
}

// DevAddSocial adds a social link in-memory and returns created Social.
func DevAddSocial(name, url, icon string, ord int) Social {
	devMu.Lock()
	defer devMu.Unlock()
	s := Social{ID: devNextSocialID, Name: name, URL: url, Icon: icon, Ord: ord}
	devNextSocialID++
	devSocials = append(devSocials, s)
	return s
}

// DevUpdateSocial updates an existing social link; returns true if found.
func DevUpdateSocial(id int64, name, url, icon string, ord int) bool {
	devMu.Lock()
	defer devMu.Unlock()
	for i := range devSocials {
		if devSocials[i].ID == id {
			devSocials[i].Name = name
			devSocials[i].URL = url
			devSocials[i].Icon = icon
			devSocials[i].Ord = ord
			return true
		}
	}
	return false
}

// DevDeleteSocial removes a social link by id and returns true if removed.
func DevDeleteSocial(id int64) bool {
	devMu.Lock()
	defer devMu.Unlock()
	idx := -1
	for i := range devSocials {
		if devSocials[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false
	}
	devSocials = append(devSocials[:idx], devSocials[idx+1:]...)
	return true
}

// DevUpdateProfile updates the in-memory profile.
func DevUpdateProfile(p Profile) {
	devMu.Lock()
	defer devMu.Unlock()
	devProfile = p
}

func DevGetCategories() []Category {
	devMu.Lock()
	defer devMu.Unlock()
	cp := make([]Category, len(devCategories))
	copy(cp, devCategories)
	return cp
}

func DevAddCategory(name string) Category {
	devMu.Lock()
	defer devMu.Unlock()
	c := Category{ID: devNextCatID, Name: name}
	devNextCatID++
	devCategories = append(devCategories, c)
	return c
}

func DevUpdateCategory(id int64, name string) bool {
	devMu.Lock()
	defer devMu.Unlock()
	for i := range devCategories {
		if devCategories[i].ID == id {
			devCategories[i].Name = name
			// update existing products using this category
			for j := range devProducts {
				if devProducts[j].CategoryID == id {
					devProducts[j].Category = name
				}
			}
			return true
		}
	}
	return false
}

func DevDeleteCategory(id int64) bool {
	devMu.Lock()
	defer devMu.Unlock()
	idx := -1
	for i, c := range devCategories {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false
	}
	devCategories = append(devCategories[:idx], devCategories[idx+1:]...)
	for j := range devProducts {
		if devProducts[j].CategoryID == id {
			devProducts[j].CategoryID = 0
			devProducts[j].Category = ""
		}
	}
	return true
}

func DevGetCategory(id int64) (Category, bool) {
	devMu.Lock()
	defer devMu.Unlock()
	for _, c := range devCategories {
		if c.ID == id {
			return c, true
		}
	}
	return Category{}, false
}
