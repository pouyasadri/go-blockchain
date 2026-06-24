# Go Blockchain

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.25-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![CI Build](https://img.shields.io/github/actions/workflow/status/pouyasadri/go-blockchain/ci.yml?branch=main&style=flat-square)](https://github.com/pouyasadri/go-blockchain/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/pouyasadri/go-blockchain?style=flat-square)](https://goreportcard.com/report/github.com/pouyasadri/go-blockchain)
[![Coverage Status](https://img.shields.io/badge/Coverage-~85%25-brightgreen.svg?style=flat-square)](https://github.com/pouyasadri/go-blockchain/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

A fully-featured, educational blockchain server and node implementation written entirely in Go. This showcase project demonstrates advanced Go system design best practices, robust testing methodologies, secure cryptographic engineering, and modern Go features (such as `iter.Seq` range-over-func).

## ✨ Features

- **Decentralized P2P Network**: Robust TCP-based server communication to propagate blocks and transactions across nodes. Includes protection against memory exhaustion DoS vectors via `io.LimitReader` and strict message size limits.
- **Context-Aware Mining Cancellation**: Instant abort of active CPU mining loops via thread-safe context cancellation when a competing valid block is received from peers, saving CPU cycles.
- **Bitcoin-Style Difficulty Retargeting**: Self-correcting difficulty adjustment. Every 10 blocks, difficulty target bits are re-calculated using block header time differences over the interval, clamped to a maximum 2x/0.5x shift to prevent sudden swings.
- **UTXO Model**: Unspent Transaction Output (UTXO) model for accurate balance checking and validation.
- **Elliptic Curve Cryptography**: Secure wallet generation and transaction signing using ECDSA (`secp256r1`) and SHA-256. Employs canonical signing and proper zero-padding fixes for deterministic signature lengths.
- **Persistent Storage**: Efficient key-value data management backed by `go.etcd.io/bbolt`, featuring atomic multi-key transactions for crash-safe state updates.
- **HTTP REST API**: Pure `net/http` JSON explorer API featuring Go 1.22+ wildcard path matching, CORS headers, and hex-serialized DTO payloads (no external routers needed).
- **Prometheus Observability**: Native Prometheus instrumentation (`blockchain_height`, `blockchain_mempool_size`, `blockchain_connected_peers`, `blockchain_mining_duration_seconds`, etc.) exposed at `/metrics`.
- **Modern CLI**: Intuitive command-line interface powered by `Cobra`.

## 🏗 System Architecture

The architecture is built using highly cohesive, decoupled domains to ensure maintainability, concurrency-safety, and testability:

```mermaid
graph TD
    subgraph CLI ["CLI Layer (internal/cli)"]
        CmdLine[Cobra CLI]
    end

    subgraph Network ["Network Layer (internal/network)"]
        P2P[P2P Server w/ sync.RWMutex]
        CmdHandler[Command Handlers]
    end

    subgraph Core ["Core Domain (internal/core)"]
        BC[Blockchain]
        PoW[Proof of Work w/ Context Cancel]
        TX[Transaction & UTXOSet]
        Wall[Wallets]
        Merkle[Merkle Trees w/ CVE-2012-2459 mitigation]
    end

    subgraph Storage ["Storage Layer (internal/storage)"]
        bbolt[go.etcd.io/bbolt]
    end

    subgraph API ["Observability & API (internal/api, metrics)"]
        APISrv[APIServer w/ net/http]
        PromMetrics[Prometheus Metrics]
    end

    CmdLine -.->|Initializes| Core
    CmdLine -.->|Starts Node & API| Network
    CmdLine -.->|Starts API| APISrv
    Network <-->|Propagates & Validates| Core
    Core <-->|Atomic Persistence| Storage
    P2P <-->|TCP P2P Protocols| ExtNodes[External Nodes]
    
    APISrv <-->|Queries State| Core
    APISrv <-->|Queries Mempool & Peers| Network
    PromMetrics -.->|Instruments| Network
    PromMetrics -.->|Exposes /metrics| APISrv
    
    APISrv <-->|JSON REST API| Clients[CURL / Browser / Dashboard]
    APISrv <-->|Scrapes Metrics| PromSystem[Prometheus / Grafana]
```

### Technical Highlights
- **Thread Safety**: The P2P node `Server` utilizes `sync.RWMutex` to protect shared state (mempool, known nodes, blocks in transit) across concurrent connection handlers, verified under the Go race detector.
- **Data Integrity**: Database interactions leverage BoltDB's atomic transactions to commit blocks and update chain tips simultaneously, eliminating the risk of torn writes during crashes.
- **Cryptographic Correctness**: Implements strict `ecdsa` signature normalization. Public keys and signature tuples (r, s) are properly zero-padded to `P-256` boundaries (32 bytes), avoiding non-deterministic decoding errors.
- **Memory Safety**: Network endpoints are secured using `io.LimitReader` to bound memory allocation during payload deserialization, thwarting resource-exhaustion attacks.

## 🚀 Getting Started

### Prerequisites

- [Go](https://go.dev/) 1.25 or later.

### Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/pouyasadri/go-blockchain.git
   cd go-blockchain
   ```

2. **Build the CLI executable:**
   ```bash
   go build -o node ./cmd/node
   ```

### Quick Start Guide

**1. Create a Wallet:**
```bash
./node createwallet
# Outputs: Your new address: <Base58 Address>
```

**2. Initialize the Blockchain:**
```bash
./node createblockchain --address <Your_Address>
```
*This generates the genesis block and mines the first reward to your wallet.*

**3. Check Balance:**
```bash
./node getbalance --address <Your_Address>
```

**4. Start the Node Server (with P2P, HTTP REST API, and Metrics):**
```bash
# Start node 3000 in mining mode with API server on port 8080
export NODE_ID=3000
./node startnode --miner <Your_Address> --api-addr :8080
```

### 🌐 Interacting with the HTTP API

Once the node is running, you can explore the state of the blockchain using curl or any web client:

- **Check Node Height:**
  ```bash
  curl http://localhost:8080/api/v1/height
  ```
- **List Blocks (Paginated):**
  ```bash
  curl "http://localhost:8080/api/v1/blocks?limit=5&offset=0"
  ```
- **Fetch Specific Block:**
  ```bash
  curl http://localhost:8080/api/v1/blocks/<block_hash_hex>
  ```
- **Get Wallet Balance:**
  ```bash
  curl http://localhost:8080/api/v1/balance/<wallet_address>
  ```
- **Get Mempool Contents:**
  ```bash
  curl http://localhost:8080/api/v1/mempool
  ```
- **Expose Prometheus Metrics:**
  ```bash
  curl http://localhost:8080/metrics
  ```

## 🧪 Testing and CI

This repository is heavily tested with end-to-end, integration, and unit coverage (maintaining **~85%** statement coverage) of core mechanics (Proof of Work, P2P network, REST API, UTXOs). To run tests locally:

```bash
# Run unit, integration, and e2e tests
go test -v ./...

# Run tests with the Go race detector enabled
go test -race ./...

# Calculate test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```
A continuous integration (CI) pipeline leverages GitHub actions to enforce code standards using `golangci-lint` and executes tests on every push or pull request to the main branch.

## 🤝 Contributing

Contributions, issues, and feature requests are always welcome! 
Please refer to the [Contributing Guide](CONTRIBUTING.md) for details on our code of conduct, pull request process, and development standards.

## 📄 License

This project is licensed under the MIT License - see the `LICENSE` file for details.
