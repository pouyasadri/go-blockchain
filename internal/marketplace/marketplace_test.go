package marketplace

import (
	"testing"

	"github.com/pouyasadri/go-blockchain/internal/core"
	"github.com/pouyasadri/go-blockchain/internal/indexer"
	"github.com/stretchr/testify/assert"
)

func TestServiceCatalogSearch(t *testing.T) {
	store := indexer.NewIndexStore()
	idx := indexer.NewIndexer(store)
	cat := NewServiceCatalog(store)

	sellerWallet, err := core.NewWallet()
	assert.NoError(t, err)
	sellerAddr := string(sellerWallet.GetAddress())

	_, err = idx.RegisterServiceOffer(sellerAddr, "Image-Generator-Fast", "Generate 512x512 PNG", "https://img.ai/api", 100)
	assert.NoError(t, err)
	_, err = idx.RegisterServiceOffer(sellerAddr, "Text-Embeddings", "Dense vector embeddings", "https://embed.ai/api", 20)
	assert.NoError(t, err)

	// Search by max price
	results := cat.Search(SearchFilter{MaxPrice: 50})
	assert.Len(t, results, 1)
	assert.Equal(t, "Text-Embeddings", results[0].Name)

	// Search by query
	results = cat.Search(SearchFilter{Query: "Image"})
	assert.Len(t, results, 1)
	assert.Equal(t, "Image-Generator-Fast", results[0].Name)
}

func TestEscrowManagerHTLCSecret(t *testing.T) {
	em := NewEscrowManager(nil)
	preimage, hashLock, err := em.CreateHTLCSecret()
	assert.NoError(t, err)
	assert.Len(t, preimage, 32)
	assert.Len(t, hashLock, 32)
}
