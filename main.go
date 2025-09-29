package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	SOL_MINT  = "So11111111111111111111111111111111111111112"
	USDC_MINT = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	JUPITER_QUOTE_API = "https://quote-api.jup.ag/v6/quote"
	JUPITER_SWAP_API  = "https://quote-api.jup.ag/v6/swap"
)

type Config struct {
	WalletAddress              string  `json:"wallet_address"`
	PrivateKey                 string  `json:"private_key"`
	RpcURL                     string  `json:"rpc_url"`
	InitialBalanceUSD          float64 `json:"initial_balance_usd"`
	PriceCheckIntervalSeconds  int     `json:"price_check_interval_seconds"`
	SwapThresholdMinPercent    float64 `json:"swap_threshold_min_percent"`
	SwapThresholdMaxPercent    float64 `json:"swap_threshold_max_percent"`
	MaxSwapsPerDay             int     `json:"max_swaps_per_day"`
	SlippageBps                int     `json:"slippage_bps"`
	SimulateMode               bool    `json:"simulate_mode"`
	PriorityFeeMicrolamports   uint64  `json:"priority_fee_microlamports"`
}


type QuoteResponse struct {
	InputMint        string `json:"inputMint"`
	InAmount         string `json:"inAmount"`
	OutputMint       string `json:"outputMint"`
	OutAmount        string `json:"outAmount"`
	OtherAmountThreshold string `json:"otherAmountThreshold"`
	SwapMode         string `json:"swapMode"`
	SlippageBps      int    `json:"slippageBps"`
}

type SwapRequest struct {
	QuoteResponse         QuoteResponse `json:"quoteResponse"`
	UserPublicKey         string        `json:"userPublicKey"`
	WrapAndUnwrapSol      bool          `json:"wrapAndUnwrapSol"`
	PriorityFeeLamports   uint64        `json:"priorityFeeLamports,omitempty"`
	DynamicComputeUnitLimit bool        `json:"dynamicComputeUnitLimit,omitempty"`
}

type SwapResponse struct {
	SwapTransaction string `json:"swapTransaction"`
	LastValidBlockHeight uint64 `json:"lastValidBlockHeight"`
}

type TradingBot struct {
	config          Config
	currentAsset    string    // "SOL" or "USDC"
	balance         float64   // Current balance in USD
	lastSwapPrice   float64   // Last price when swap occurred
	swapCount       int       // Number of swaps today
	lastSwapReset   time.Time // Last time swap count was reset
	rpcClient       *rpc.Client
	wallet          solana.PrivateKey
}

func NewTradingBot(config Config) (*TradingBot, error) {
	// Initialize Solana RPC client
	rpcClient := rpc.New(config.RpcURL)

	// Parse base58 private key
	privateKey, err := solana.PrivateKeyFromBase58(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid base58 private key: %w", err)
	}
	log.Printf("Successfully loaded wallet from base58 private key")

	// Verify public key matches wallet address (if provided)
	expectedPubkey := privateKey.PublicKey()
	if config.WalletAddress != "" && expectedPubkey.String() != config.WalletAddress {
		log.Printf("Warning: Generated public key (%s) doesn't match provided wallet address (%s)",
			expectedPubkey.String(), config.WalletAddress)
		log.Printf("Using generated public key: %s", expectedPubkey.String())
	}

	return &TradingBot{
		config:        config,
		currentAsset:  "SOL",
		balance:       config.InitialBalanceUSD,
		lastSwapPrice: 0,
		swapCount:     0,
		lastSwapReset: time.Now(),
		rpcClient:     rpcClient,
		wallet:        privateKey,
	}, nil
}

func loadConfig(filename string) (Config, error) {
	var config Config

	file, err := os.ReadFile(filename)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	err = json.Unmarshal(file, &config)
	if err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

func (tb *TradingBot) resetDailySwapCount() {
	now := time.Now()
	if now.Sub(tb.lastSwapReset) >= 24*time.Hour {
		tb.swapCount = 0
		tb.lastSwapReset = now
		log.Println("Daily swap count reset")
	}
}

func getSolanaPrice(slippageBps int) (float64, error) {
	// Get quote for 1 SOL to USDC
	url := fmt.Sprintf("%s?inputMint=%s&outputMint=%s&amount=1000000000&slippageBps=%d",
		JUPITER_QUOTE_API, SOL_MINT, USDC_MINT, slippageBps)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to get quote: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	var quote QuoteResponse
	err = json.Unmarshal(body, &quote)
	if err != nil {
		return 0, fmt.Errorf("failed to parse quote response: %w", err)
	}

	// Convert outAmount from lamports to USDC (6 decimals)
	outAmountInt, err := strconv.ParseInt(quote.OutAmount, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse output amount: %w", err)
	}

	price := float64(outAmountInt) / 1000000 // USDC has 6 decimals
	return price, nil
}

func (tb *TradingBot) shouldSwap(currentPrice float64) (bool, string) {
	if tb.lastSwapPrice == 0 {
		tb.lastSwapPrice = currentPrice
		return false, "Initial price set"
	}

	priceChangePercent := ((currentPrice - tb.lastSwapPrice) / tb.lastSwapPrice) * 100

	minThreshold := tb.config.SwapThresholdMinPercent
	//maxThreshold := tb.config.SwapThresholdMaxPercent

	// If holding SOL and price increased by at least minimum threshold
	if tb.currentAsset == "SOL" && priceChangePercent >= minThreshold {
		return true, fmt.Sprintf("SOL price increased by %.2f%% (%.2f -> %.2f), swapping to USDC",
			priceChangePercent, tb.lastSwapPrice, currentPrice)
	}

	// If holding USDC and price decreased by at least minimum threshold
	if tb.currentAsset == "USDC" && priceChangePercent <= -minThreshold {
		return true, fmt.Sprintf("SOL price decreased by %.2f%% (%.2f -> %.2f), swapping to SOL",
			priceChangePercent, tb.lastSwapPrice, currentPrice)
	}

	return false, fmt.Sprintf("Price change %.2f%% doesn't meet swap criteria", priceChangePercent)
}

func (tb *TradingBot) executeSwap(currentPrice float64) error {
	// Check daily swap limit
	if tb.swapCount >= tb.config.MaxSwapsPerDay {
		return fmt.Errorf("daily swap limit reached (%d/%d)", tb.swapCount, tb.config.MaxSwapsPerDay)
	}

	var fromMint, toMint string
	var amount int64

	if tb.currentAsset == "SOL" {
		// Swap SOL to USDC
		fromMint = SOL_MINT
		toMint = USDC_MINT
		// Convert USD balance to SOL lamports (9 decimals)
		amount = int64((tb.balance / currentPrice) * 1000000000)
	} else {
		// Swap USDC to SOL
		fromMint = USDC_MINT
		toMint = SOL_MINT
		// Convert USD balance to USDC (6 decimals)
		amount = int64(tb.balance * 1000000)
	}

	// Get quote for the swap
	quoteURL := fmt.Sprintf("%s?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d",
		JUPITER_QUOTE_API, fromMint, toMint, amount, tb.config.SlippageBps)

	resp, err := http.Get(quoteURL)
	if err != nil {
		return fmt.Errorf("failed to get swap quote: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read quote response: %w", err)
	}

	var quote QuoteResponse
	err = json.Unmarshal(body, &quote)
	if err != nil {
		return fmt.Errorf("failed to parse quote response: %w", err)
	}

	log.Printf("Swap quote received: %s %s -> %s %s",
		quote.InAmount, quote.InputMint[:8]+"...",
		quote.OutAmount, quote.OutputMint[:8]+"...")

	if tb.config.SimulateMode {
		// Simulation mode - don't actually execute swap
		log.Printf("SIMULATED SWAP: %s -> %s", tb.currentAsset,
			map[string]string{"SOL": "USDC", "USDC": "SOL"}[tb.currentAsset])
	} else {
		// Execute real swap
		log.Printf("EXECUTING REAL SWAP: %s -> %s", tb.currentAsset,
			map[string]string{"SOL": "USDC", "USDC": "SOL"}[tb.currentAsset])

		err := tb.executeRealSwap(quote)
		if err != nil {
			return fmt.Errorf("real swap execution failed: %w", err)
		}
		log.Printf("Swap executed successfully!")
	}

	// Update bot state
	if tb.currentAsset == "SOL" {
		tb.currentAsset = "USDC"
		// Add 3% profit (simulate successful trade)
		tb.balance = tb.balance * 1.03
	} else {
		tb.currentAsset = "SOL"
	}

	tb.lastSwapPrice = currentPrice
	tb.swapCount++

	log.Printf("New balance: $%.2f in %s", tb.balance, tb.currentAsset)
	log.Printf("Swaps today: %d/%d", tb.swapCount, tb.config.MaxSwapsPerDay)

	return nil
}

func (tb *TradingBot) executeRealSwap(quote QuoteResponse) error {
	// Create swap request
	swapReq := SwapRequest{
		QuoteResponse:         quote,
		UserPublicKey:         tb.wallet.PublicKey().String(),
		WrapAndUnwrapSol:      true,
		PriorityFeeLamports:   tb.config.PriorityFeeMicrolamports,
		DynamicComputeUnitLimit: true,
	}

	// Marshal swap request to JSON
	swapReqBytes, err := json.Marshal(swapReq)
	if err != nil {
		return fmt.Errorf("failed to marshal swap request: %w", err)
	}

	// Call Jupiter Swap API
	resp, err := http.Post(JUPITER_SWAP_API, "application/json", bytes.NewBuffer(swapReqBytes))
	if err != nil {
		return fmt.Errorf("failed to call Jupiter swap API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jupiter swap API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse swap response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read swap response: %w", err)
	}

	var swapResp SwapResponse
	err = json.Unmarshal(body, &swapResp)
	if err != nil {
		return fmt.Errorf("failed to parse swap response: %w", err)
	}

	// Decode the transaction
	txBytes, err := base64.StdEncoding.DecodeString(swapResp.SwapTransaction)
	if err != nil {
		return fmt.Errorf("failed to decode transaction: %w", err)
	}

	// Parse transaction
	tx, err := solana.TransactionFromBytes(txBytes)
	if err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}

	// Sign the transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(tb.wallet.PublicKey()) {
			return &tb.wallet
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sig, err := tb.rpcClient.SendTransaction(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to send transaction: %w", err)
	}

	log.Printf("Transaction sent: %s", sig.String())

	// Wait for confirmation
	return tb.waitForConfirmation(ctx, sig, swapResp.LastValidBlockHeight)
}

func (tb *TradingBot) waitForConfirmation(ctx context.Context, signature solana.Signature, lastValidBlockHeight uint64) error {
	log.Printf("Waiting for transaction confirmation...")

	// Get current block height (for reference)
	_, err := tb.rpcClient.GetBlockHeight(ctx, rpc.CommitmentFinalized)
	if err != nil {
		log.Printf("Warning: failed to get initial block height: %v", err)
	}

	// Wait for confirmation with timeout based on block height
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("transaction confirmation timeout")
		case <-ticker.C:
			// Check transaction status
			status, err := tb.rpcClient.GetSignatureStatuses(ctx, true, signature)
			if err != nil {
				log.Printf("Error checking transaction status: %v", err)
				continue
			}

			if len(status.Value) > 0 && status.Value[0] != nil {
				txStatus := status.Value[0]
				if txStatus.Err != nil {
					return fmt.Errorf("transaction failed: %v", txStatus.Err)
				}
				if txStatus.ConfirmationStatus != "" {
					log.Printf("Transaction confirmed with status: %s", txStatus.ConfirmationStatus)
					return nil
				}
			}

			// Check if we've exceeded the last valid block height
			currentHeight, err := tb.rpcClient.GetBlockHeight(ctx, rpc.CommitmentFinalized)
			if err != nil {
				log.Printf("Error getting current block height: %v", err)
				continue
			}

			if currentHeight > lastValidBlockHeight {
				return fmt.Errorf("transaction expired (current block: %d, last valid: %d)",
					currentHeight, lastValidBlockHeight)
			}
		}
	}
}

func (tb *TradingBot) getSOLBalance() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the wallet address from config, not derived address
	walletPubkey := solana.MustPublicKeyFromBase58(tb.config.WalletAddress)
	balance, err := tb.rpcClient.GetBalance(ctx, walletPubkey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("failed to get SOL balance: %w", err)
	}

	// Convert lamports to SOL (9 decimals)
	solBalance := float64(balance.Value) / 1000000000
	return solBalance, nil
}

func (tb *TradingBot) getUSDCBalance() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the wallet address from config, not derived address
	walletPubkey := solana.MustPublicKeyFromBase58(tb.config.WalletAddress)

	// Get token accounts for USDC
	usdcMint := solana.MustPublicKeyFromBase58(USDC_MINT)
	tokenAccounts, err := tb.rpcClient.GetTokenAccountsByOwner(ctx, walletPubkey, &rpc.GetTokenAccountsConfig{
		Mint: &usdcMint,
	}, &rpc.GetTokenAccountsOpts{
		Commitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get USDC token accounts: %w", err)
	}

	if len(tokenAccounts.Value) == 0 {
		return 0, nil // No USDC token account found
	}

	// Get balance from first token account
	tokenAccount := tokenAccounts.Value[0]
	balance, err := tb.rpcClient.GetTokenAccountBalance(ctx, tokenAccount.Pubkey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("failed to get USDC balance: %w", err)
	}

	// Convert to USDC (6 decimals)
	amount, err := strconv.ParseFloat(balance.Value.Amount, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse USDC amount: %w", err)
	}
	usdcBalance := amount / 1000000
	return usdcBalance, nil
}

func (tb *TradingBot) getCurrentBalanceUSD(solPrice float64) (float64, string, error) {
	solBalance, err := tb.getSOLBalance()
	if err != nil {
		return 0, "", err
	}

	usdcBalance, err := tb.getUSDCBalance()
	if err != nil {
		return 0, "", err
	}

	solUSD := solBalance * solPrice

	// Determine which asset we're primarily holding
	if solUSD > usdcBalance {
		return solUSD, "SOL", nil
	} else if usdcBalance > 0 {
		return usdcBalance, "USDC", nil
	} else {
		return solUSD, "SOL", nil
	}
}

func (tb *TradingBot) run() {
	log.Printf("Starting Solana Trading Bot")
	log.Printf("Initial balance: $%.2f in %s", tb.balance, tb.currentAsset)
	log.Printf("Wallet address: %s", tb.wallet.PublicKey().String())
	log.Printf("Price check interval: %ds", tb.config.PriceCheckIntervalSeconds)
	log.Printf("Swap thresholds: %.1f%% - %.1f%%", tb.config.SwapThresholdMinPercent, tb.config.SwapThresholdMaxPercent)
	log.Printf("Max swaps per day: %d", tb.config.MaxSwapsPerDay)
	log.Printf("Simulate mode: %t", tb.config.SimulateMode)

	ticker := time.NewTicker(time.Duration(tb.config.PriceCheckIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tb.resetDailySwapCount()

			price, err := getSolanaPrice(tb.config.SlippageBps)
			if err != nil {
				log.Printf("Error getting SOL price: %v", err)
				continue
			}

			// Get real balance from blockchain
			realBalance, currentAsset, err := tb.getCurrentBalanceUSD(price)
			if err != nil {
				log.Printf("Error getting real balance: %v", err)
				continue
			}

			// Update bot state with real balance
			tb.balance = realBalance
			tb.currentAsset = currentAsset

			log.Printf("Current SOL price: $%.2f | Holding: %s ($%.2f) | Last swap: $%.2f",
				price, tb.currentAsset, tb.balance, tb.lastSwapPrice)

			shouldSwap, reason := tb.shouldSwap(price)
			log.Printf("Swap decision: %s", reason)

			if shouldSwap {
				err := tb.executeSwap(price)
				if err != nil {
					log.Printf("Swap failed: %v", err)
				}
			}
		}
	}
}

func main() {
	// Load configuration
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate configuration
	if config.PrivateKey == "YOUR_PRIVATE_KEY_HERE" || config.PrivateKey == "" {
		log.Fatal("Please set your private key or seed phrase in config.json")
	}

	// Create and start trading bot
	bot, err := NewTradingBot(config)
	if err != nil {
		log.Fatalf("Failed to create trading bot: %v", err)
	}

	bot.run()
}