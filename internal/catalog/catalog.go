// Package catalog loads store catalogs from per-store JSON files.
//
// Layout: one file per store, named <store-id>.json, inside a catalogs dir:
//
//	catalogs/demo-store.json
//
// The intelligence layer (the customer's AI) does all filtering; this package
// only parses and hands back clean structured data.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Product is a catalog item. The schema is provisional — to be refined with a
// real store — so keep field names stable and additive going forward.
type Product struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Price       float64  `json:"price"`
	Currency    string   `json:"currency"`
	Sizes       []string `json:"sizes,omitempty"`
	InStock     bool     `json:"in_stock"`
	ImageURL    string   `json:"image_url,omitempty"`
	ProductURL  string   `json:"product_url,omitempty"`
}

type fileFormat struct {
	Name     string    `json:"name"`
	Products []Product `json:"products"`
}

// Catalog is a loaded store catalog.
type Catalog struct {
	StoreID   string
	StoreName string
	Products  []Product
}

// LoadDir loads the catalog for storeID from dir, i.e. dir/<storeID>.json.
func LoadDir(dir, storeID string) (*Catalog, error) {
	return Load(filepath.Join(dir, storeID+".json"), storeID)
}

// Load reads a catalog file, attributing it to storeID.
func Load(path, storeID string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}

	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse catalog %s: %w", path, err)
	}
	if len(f.Products) == 0 {
		return nil, fmt.Errorf("catalog %s has no products", path)
	}

	c := &Catalog{StoreID: storeID, StoreName: f.Name, Products: f.Products}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("catalog %s: %w", path, err)
	}
	return c, nil
}

// validate enforces minimal invariants so the AI consumer never sees garbage.
func (c *Catalog) validate() error {
	if c.StoreName == "" {
		return fmt.Errorf("missing store name")
	}
	seen := map[string]bool{}
	for _, p := range c.Products {
		if p.ID == "" {
			return fmt.Errorf("product with empty id: %q", p.Name)
		}
		if seen[p.ID] {
			return fmt.Errorf("duplicate product id %q", p.ID)
		}
		seen[p.ID] = true
	}
	return nil
}
