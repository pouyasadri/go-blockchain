package marketplace

import (
	"fmt"
	"strings"

	"github.com/pouyasadri/go-blockchain/internal/indexer"
)

// ServiceCatalog wraps indexer service queries with filtering and ranking logic
type ServiceCatalog struct {
	Store *indexer.IndexStore
}

// NewServiceCatalog creates a new service catalog instance
func NewServiceCatalog(store *indexer.IndexStore) *ServiceCatalog {
	return &ServiceCatalog{
		Store: store,
	}
}

// SearchFilter specifies search criteria for discovering AI services
type SearchFilter struct {
	Query        string `json:"query,omitempty"`
	MaxPrice     int64  `json:"max_price,omitempty"`
	AgentAddress string `json:"agent_address,omitempty"`
}

// Search finds matching services based on the filter
func (sc *ServiceCatalog) Search(filter SearchFilter) []*indexer.AgentService {
	allServices := sc.Store.ListServices()
	var results []*indexer.AgentService

	queryLower := strings.ToLower(filter.Query)

	for _, srv := range allServices {
		if filter.MaxPrice > 0 && srv.PricePerCall > filter.MaxPrice {
			continue
		}
		if filter.AgentAddress != "" && srv.AgentAddress != filter.AgentAddress {
			continue
		}
		if queryLower != "" {
			nameLower := strings.ToLower(srv.Name)
			descLower := strings.ToLower(srv.Description)
			if !strings.Contains(nameLower, queryLower) && !strings.Contains(descLower, queryLower) {
				continue
			}
		}
		results = append(results, srv)
	}

	return results
}

// GetServiceByID retrieves a specific service
func (sc *ServiceCatalog) GetServiceByID(id string) (*indexer.AgentService, error) {
	srv, ok := sc.Store.GetService(id)
	if !ok {
		return nil, fmt.Errorf("service with ID %s not found", id)
	}
	return srv, nil
}
