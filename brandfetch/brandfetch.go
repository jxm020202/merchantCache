package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type SearchHit struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Domain       string   `json:"domain"`
	QualityScore float64  `json:"qualityScore"`
	Aliases      []string `json:"aliases"`
}

type BrandProfile struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Domain       string  `json:"domain"`
	QualityScore float64 `json:"qualityScore"`
	Company      Company `json:"company"`
	Raw          json.RawMessage
}

type Company struct {
	Location Location `json:"location"`
}

type Location struct {
	City        string `json:"city"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

type choiceProfile struct {
	ID           string
	Name         string
	Domain       string
	QualityScore float64
}

func searchBrand(ctx context.Context, client *http.Client, name string, cfg Config) (*SearchHit, error) {
	url := fmt.Sprintf("https://api.brandfetch.io/v2/search/%s?c=%s", urlEncode(name), cfg.BrandfetchClientID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search status %d", resp.StatusCode)
	}
	var hits []SearchHit
	if err := json.NewDecoder(resp.Body).Decode(&hits); err != nil {
		return nil, err
	}
	return pickPreferredHit(hits, cfg.CountryTLDPreference), nil
}

func fetchBrandProfile(ctx context.Context, client *http.Client, domain string, cfg Config) (*BrandProfile, error) {
	if domain == "" {
		return nil, nil
	}
	url := fmt.Sprintf("https://api.brandfetch.io/v2/brands/%s", urlEncode(domain))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.BrandfetchAPIKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brand api status %d", resp.StatusCode)
	}
	var prof BrandProfile
	body, err := ioReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &prof); err != nil {
		return nil, err
	}
	prof.Raw = body
	return &prof, nil
}

func pickPreferredHit(hits []SearchHit, tld string) *SearchHit {
	if len(hits) == 0 {
		return nil
	}
	if tld != "" {
		for _, h := range hits {
			if strings.HasSuffix(h.Domain, tld) {
				return &h
			}
		}
	}
	return &hits[0]
}

func pickProfile(profile *BrandProfile, hit *SearchHit) choiceProfile {
	if profile != nil {
		return choiceProfile{
			ID:           profile.ID,
			Name:         profile.Name,
			Domain:       profile.Domain,
			QualityScore: profile.QualityScore,
		}
	}
	if hit != nil {
		return choiceProfile{
			ID:           hit.ID,
			Name:         hit.Name,
			Domain:       hit.Domain,
			QualityScore: hit.QualityScore,
		}
	}
	return choiceProfile{}
}

func rawJSON(profile *BrandProfile, hit *SearchHit) json.RawMessage {
	if profile != nil && profile.Raw != nil {
		return profile.Raw
	}
	if hit != nil {
		b, _ := json.Marshal(hit)
		return b
	}
	return json.RawMessage(`null`)
}

func domainToURL(domain string) string {
	if domain == "" {
		return ""
	}
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return domain
	}
	return "https://" + domain
}

func logoURL(domain, clientID string) string {
	if domain == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.brandfetch.io/%s/theme/dark/logo?c=%s", domain, clientID)
}

func urlEncode(s string) string {
	return url.QueryEscape(strings.TrimSpace(s))
}

func ioReadAll(r io.Reader) ([]byte, error) {
	const max = 4 << 20 // 4MB safety
	var b []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if len(b)+n > max {
				return nil, fmt.Errorf("response too large")
			}
			b = append(b, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return b, nil
			}
			return nil, err
		}
	}
}
