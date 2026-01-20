package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"merchantcache/internal/abr"
	"merchantcache/internal/google"
)

type MerchantResult struct {
	Name            string
	GoogleLegalName string
	GoogleState     string
	GooglePostcode  string
	ABNFound        string
	ABNLegalName    string
	ABNState        string
	Verified        string
	ABNCount        int
	AllABNResults   []abr.Result
}

type Server struct {
	googleClient *google.Client
	abrClient    *abr.Client
	csvFile      string
	results      map[string]MerchantResult
	cache        map[string]MerchantResult
	cacheMutex   sync.RWMutex
}

func NewServer(googleClient *google.Client, abrClient *abr.Client, csvFile string) *Server {
	return &Server{
		googleClient: googleClient,
		abrClient:    abrClient,
		csvFile:      csvFile,
		results:      make(map[string]MerchantResult),
		cache:        make(map[string]MerchantResult),
	}
}

func (s *Server) LoadResults() error {
	file, err := os.Open(s.csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	// Skip header
	for _, record := range records[1:] {
		if len(record) < 5 {
			continue
		}
		s.results[record[0]] = MerchantResult{
			Name:         record[0],
			ABNFound:     record[1],
			ABNState:     record[2],
			ABNLegalName: record[3],
			Verified:     record[5],
		}
	}
	return nil
}

func (s *Server) Start(port string) error {
	http.HandleFunc("/api/merchant/", s.handleMerchantAPI)
	http.HandleFunc("/api/search/", s.handleSearchAPI)
	http.HandleFunc("/health", s.handleHealth)

	fmt.Printf("Starting API server on http://localhost:%s\n", port)
	return http.ListenAndServe(":"+port, nil)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok"}`)
}

func (s *Server) handleMerchantAPI(w http.ResponseWriter, r *http.Request) {
	merchantName := strings.TrimPrefix(r.URL.Path, "/api/merchant/")
	merchantName = strings.Trim(merchantName, "/")

	result, exists := s.results[merchantName]
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error": "merchant not found"}`)
		return
	}

	// Check cache first
	s.cacheMutex.RLock()
	cachedResult, cached := s.cache[merchantName]
	s.cacheMutex.RUnlock()

	if cached {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Name":             cachedResult.Name,
			"GoogleLegalName":  cachedResult.GoogleLegalName,
			"GoogleState":      cachedResult.GoogleState,
			"GooglePostcode":   cachedResult.GooglePostcode,
			"ABNFound":         cachedResult.ABNFound,
			"ABNLegalName":     cachedResult.ABNLegalName,
			"ABNState":         cachedResult.ABNState,
			"Verified":         cachedResult.Verified,
			"ABNCount":         cachedResult.ABNCount,
			"AllABNResults":    cachedResult.AllABNResults,
		})
		return
	}

	// Get all ABN results for this merchant
	allResults := s.abrClient.GetAllResults(merchantName)
	result.AllABNResults = allResults
	result.ABNCount = len(allResults)

	// Get Google info
	googleInfo, _ := s.googleClient.ExtractMerchantInfo(merchantName)
	result.GoogleLegalName = googleInfo.LegalName
	result.GoogleState = googleInfo.State
	result.GooglePostcode = googleInfo.Postcode

	// Cache the result
	s.cacheMutex.Lock()
	s.cache[merchantName] = result
	s.cacheMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"Name":             result.Name,
		"GoogleLegalName":  result.GoogleLegalName,
		"GoogleState":      result.GoogleState,
		"GooglePostcode":   result.GooglePostcode,
		"ABNFound":         result.ABNFound,
		"ABNLegalName":     result.ABNLegalName,
		"ABNState":         result.ABNState,
		"Verified":         result.Verified,
		"ABNCount":         result.ABNCount,
		"AllABNResults":    result.AllABNResults,
	})
}

func (s *Server) handleSearchAPI(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var filtered []MerchantResult
	for _, result := range s.results {
		if query == "*" || strings.Contains(strings.ToLower(result.Name), strings.ToLower(query)) {
			filtered = append(filtered, result)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}
