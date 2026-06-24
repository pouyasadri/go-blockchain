package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/pouyasadri/go-blockchain/internal/api"
	"github.com/pouyasadri/go-blockchain/internal/core"
	"github.com/pouyasadri/go-blockchain/internal/network"
	"github.com/pouyasadri/go-blockchain/internal/storage/bolt"
	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(args []string, out io.Writer) error {
	var nodeID string

	rootCmd := &cobra.Command{
		Use:   "node",
		Short: "A simple blockchain node",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			nodeID = os.Getenv("NODE_ID")
			if nodeID == "" {
				return fmt.Errorf("NODE_ID env. var is not set")
			}
			return nil
		},
	}
	rootCmd.SetArgs(args)
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)

	createBlockchainCmd := &cobra.Command{
		Use:   "createblockchain",
		Short: "Create a blockchain and send genesis block reward to ADDRESS",
		RunE: func(cmd *cobra.Command, args []string) error {
			address, _ := cmd.Flags().GetString("address")
			if address == "" {
				return cmd.Help()
			}

			dbFile := fmt.Sprintf("blockchain_%s.db", nodeID)
			if _, err := os.Stat(dbFile); err == nil {
				return fmt.Errorf("blockchain already exists")
			}

			db, err := bolt.Open(dbFile)
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer func() {
				_ = db.Close()
			}()

			bc, err := core.CreateBlockchain(address, db)
			if err != nil {
				return fmt.Errorf("failed to create blockchain: %w", err)
			}

			UTXOSet := core.UTXOSet{Blockchain: bc}
			err = UTXOSet.Reindex()
			if err != nil {
				return fmt.Errorf("failed to reindex utxo set: %w", err)
			}

			cmd.Println("Done!")
			return nil
		},
	}
	createBlockchainCmd.Flags().String("address", "", "The address to send genesis block reward to")

	createWalletCmd := &cobra.Command{
		Use:   "createwallet",
		Short: "Generates a new key-pair and saves it into the wallet file",
		RunE: func(cmd *cobra.Command, args []string) error {
			wallets, err := core.NewWallets(nodeID)
			if err != nil {
				return fmt.Errorf("failed to load wallets: %w", err)
			}
			address, err := wallets.CreateWallet()
			if err != nil {
				return fmt.Errorf("failed to create wallet: %w", err)
			}
			err = wallets.SaveToFile(nodeID)
			if err != nil {
				return fmt.Errorf("failed to save wallets: %w", err)
			}

			cmd.Printf("Your new address: %s\n", address)
			return nil
		},
	}

	getBalanceCmd := &cobra.Command{
		Use:   "getbalance",
		Short: "Get balance of ADDRESS",
		RunE: func(cmd *cobra.Command, args []string) error {
			address, _ := cmd.Flags().GetString("address")
			if address == "" {
				return cmd.Help()
			}
			if !core.ValidateAddress(address) {
				return fmt.Errorf("ERROR: Address is not valid")
			}

			db, err := bolt.Open(fmt.Sprintf("blockchain_%s.db", nodeID))
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer func() {
				_ = db.Close()
			}()

			bc, err := core.NewBlockchain(db)
			if err != nil {
				return fmt.Errorf("failed to open blockchain: %w", err)
			}

			UTXOSet := core.UTXOSet{Blockchain: bc}
			pubKeyHash := core.Base58Decode([]byte(address))
			pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
			UTXOs, err := UTXOSet.FindUTXO(pubKeyHash)
			if err != nil {
				return fmt.Errorf("failed to find utxos: %w", err)
			}

			balance := 0
			for _, out := range UTXOs {
				balance += out.Value
			}

			cmd.Printf("Balance of '%s': %d\n", address, balance)
			return nil
		},
	}
	getBalanceCmd.Flags().String("address", "", "The address to get balance for")

	listAddressesCmd := &cobra.Command{
		Use:   "listaddresses",
		Short: "Lists all addresses from the wallet file",
		RunE: func(cmd *cobra.Command, args []string) error {
			wallets, err := core.NewWallets(nodeID)
			if err != nil {
				return fmt.Errorf("failed to load wallets: %w", err)
			}
			addresses := wallets.GetAddresses()

			for _, address := range addresses {
				cmd.Println(address)
			}
			return nil
		},
	}

	printChainCmd := &cobra.Command{
		Use:   "printchain",
		Short: "Print all the blocks of the blockchain",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := bolt.Open(fmt.Sprintf("blockchain_%s.db", nodeID))
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer func() {
				_ = db.Close()
			}()

			bc, err := core.NewBlockchain(db)
			if err != nil {
				return fmt.Errorf("failed to open blockchain: %w", err)
			}

			for block := range bc.Blocks() {
				cmd.Printf("============ Block %x ============\n", block.Hash)
				cmd.Printf("Height: %d\n", block.Height)
				cmd.Printf("Prev. block: %x\n", block.PrevBlockHash)
				pow := core.NewProofOfWork(block)
				cmd.Printf("PoW: %s\n\n", strconv.FormatBool(pow.Validate()))
			}
			return nil
		},
	}

	reindexUTXOCmd := &cobra.Command{
		Use:   "reindexutxo",
		Short: "Rebuilds the UTXO set",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := bolt.Open(fmt.Sprintf("blockchain_%s.db", nodeID))
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer func() {
				_ = db.Close()
			}()

			bc, err := core.NewBlockchain(db)
			if err != nil {
				return fmt.Errorf("failed to open blockchain: %w", err)
			}

			UTXOSet := core.UTXOSet{Blockchain: bc}
			err = UTXOSet.Reindex()
			if err != nil {
				return fmt.Errorf("failed to reindex utxo set: %w", err)
			}

			count, err := UTXOSet.CountTransactions()
			if err != nil {
				return fmt.Errorf("failed to count transactions: %w", err)
			}

			cmd.Printf("Done! There are %d transactions in the UTXO set.\n", count)
			return nil
		},
	}

	sendCmd := &cobra.Command{
		Use:   "send",
		Short: "Send AMOUNT of coins from FROM address to TO. Mine on the same node, when -mine is set.",
		RunE: func(cmd *cobra.Command, args []string) error {
			from, _ := cmd.Flags().GetString("from")
			to, _ := cmd.Flags().GetString("to")
			amount, _ := cmd.Flags().GetInt("amount")
			mineNow, _ := cmd.Flags().GetBool("mine")

			if from == "" || to == "" || amount <= 0 {
				return cmd.Help()
			}
			if !core.ValidateAddress(from) {
				return fmt.Errorf("ERROR: Sender address is not valid")
			}
			if !core.ValidateAddress(to) {
				return fmt.Errorf("ERROR: Recipient address is not valid")
			}

			db, err := bolt.Open(fmt.Sprintf("blockchain_%s.db", nodeID))
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer func() {
				_ = db.Close()
			}()

			bc, err := core.NewBlockchain(db)
			if err != nil {
				return fmt.Errorf("failed to open blockchain: %w", err)
			}

			UTXOSet := core.UTXOSet{Blockchain: bc}

			wallets, err := core.NewWallets(nodeID)
			if err != nil {
				return fmt.Errorf("failed to load wallets: %w", err)
			}
			wallet, err := wallets.GetWallet(from)
			if err != nil {
				return fmt.Errorf("failed to get wallet: %w", err)
			}

			tx, err := core.NewUTXOTransaction(&wallet, to, amount, &UTXOSet)
			if err != nil {
				return fmt.Errorf("failed to create tx: %w", err)
			}

			if mineNow {
				cbTx, err := core.NewCoinbaseTX(from, "")
				if err != nil {
					return fmt.Errorf("failed to create coinbase tx: %w", err)
				}
				txs := []*core.Transaction{cbTx, tx}

				newBlock, err := bc.MineBlock(cmd.Context(), txs)
				if err != nil {
					return fmt.Errorf("failed to mine block: %w", err)
				}
				err = UTXOSet.Update(newBlock)
				if err != nil {
					return fmt.Errorf("failed to update utxo set: %w", err)
				}
				cmd.Println("Success!")
			} else {
				logger := slog.New(slog.NewTextHandler(out, nil))
				server := network.NewServer(nodeID, "", bc, logger)
				server.SendTxToKnownNodes(tx)
				cmd.Println("Success!")
			}
			return nil
		},
	}
	sendCmd.Flags().String("from", "", "Source wallet address")
	sendCmd.Flags().String("to", "", "Destination wallet address")
	sendCmd.Flags().Int("amount", 0, "Amount to send")
	sendCmd.Flags().Bool("mine", false, "Mine immediately on the same node")

	startNodeCmd := &cobra.Command{
		Use:   "startnode",
		Short: "Start a node with ID specified in NODE_ID env. var. -miner enables mining",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			miner, _ := cmd.Flags().GetString("miner")
			apiAddr, _ := cmd.Flags().GetString("api-addr")

			if miner != "" && !core.ValidateAddress(miner) {
				return fmt.Errorf("ERROR: Miner address is not valid")
			}

			db, err := bolt.Open(fmt.Sprintf("blockchain_%s.db", nodeID))
			if err != nil {
				return fmt.Errorf("failed to open storage: %w", err)
			}
			defer func() {
				_ = db.Close()
			}()

			bc, err := core.NewBlockchain(db)
			if err != nil {
				return fmt.Errorf("failed to open blockchain: %w", err)
			}

			logger := slog.New(slog.NewTextHandler(out, nil))
			server := network.NewServer(nodeID, miner, bc, logger)

			apiServer := api.NewAPIServer(bc, server)
			go func() {
				logger.Info("starting API server", "addr", apiAddr)
				if err := apiServer.Start(ctx, apiAddr); err != nil {
					logger.Error("API server error", "error", err)
				}
			}()

			server.Start(ctx)
			return nil
		},
	}
	startNodeCmd.Flags().String("miner", "", "Enable mining mode and send reward to ADDRESS")
	startNodeCmd.Flags().String("api-addr", ":8080", "Address to run the HTTP API server on")

	rootCmd.AddCommand(createBlockchainCmd, createWalletCmd, getBalanceCmd, listAddressesCmd, printChainCmd, reindexUTXOCmd, sendCmd, startNodeCmd)

	return rootCmd.Execute()
}
