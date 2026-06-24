package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	BlockchainHeight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "blockchain_height",
		Help: "Current block height of the local tip",
	})

	MempoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "blockchain_mempool_size",
		Help: "Number of unconfirmed transactions in the local mempool",
	})

	ConnectedPeers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "blockchain_connected_peers",
		Help: "Number of known peers in the network",
	})

	MiningDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "blockchain_mining_duration_seconds",
		Help:    "Time taken to mine a new block",
		Buckets: prometheus.DefBuckets,
	})

	BlocksMined = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blockchain_blocks_mined_total",
		Help: "Total number of blocks mined by this node",
	})

	TransactionsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blockchain_transactions_received_total",
		Help: "Total number of transactions received by this node",
	})
)
