# Solana Trading Bot

An automated trading bot that monitors Solana (SOL) prices and executes real swaps based on price movements using Jupiter's API.

## ✨ Features

- **🔄 Real Swap Execution**: Uses Jupiter API for actual token swaps
- **📊 Price Monitoring**: Checks SOL/USDC price every 10 seconds
- **🧠 Smart Trading Logic**:
  - Swaps SOL → USDC when price increases 3-5%
  - Swaps USDC → SOL when price decreases 3-5%
- **⚡ Risk Management**:
  - Maximum 3 swaps per 24-hour period
  - Configurable slippage protection
  - Transaction confirmation monitoring
- **🔐 Secure Wallet Integration**: Direct private key signing
- **📈 Balance Tracking**: Real-time USD balance tracking

## 🎯 Trading Strategy

1. **Initial State**: Start with $10 worth of SOL
2. **Price Increase (3-5%)**: Swap SOL → USDC to lock in profits
3. **Price Decrease (3-5%)**: Swap USDC → SOL to buy the dip
4. **Daily Limits**: Maximum 3 swaps per day to prevent overtrading
5. **Continuous Monitoring**: 10-second price check intervals

## 🚀 Setup

### 1. Install Dependencies
```bash
# Install Go (version 1.21+)
go mod tidy
```

### 2. Configure Your Wallet

Edit `config.json`:
```json
{
  "wallet_address": "YOUR_SOLANA_WALLET_ADDRESS",
  "private_key": "YOUR_PRIVATE_KEY_BASE58",
  "rpc_url": "https://api.mainnet-beta.solana.com",
  "initial_balance_usd": 10.0,
  "price_check_interval_seconds": 10,
  "swap_threshold_min_percent": 3.0,
  "swap_threshold_max_percent": 5.0,
  "max_swaps_per_day": 3,
  "slippage_bps": 50,
  "simulate_mode": true,
  "priority_fee_microlamports": 10000
}
```

### 3. Get Your Private Key

**⚠️ SECURITY WARNING**: Never share your private key. Store it securely.

From Solana CLI:
```bash
solana-keygen show --keypair ~/.config/solana/id.json
```

From Phantom/Solflare: Export private key (base58 format)

### 4. Test Mode First
Keep `"simulate_mode": true` for testing, then set to `false` for real trading.

### 5. Run the Bot
```bash
go build && ./solana-trading-bot
```

## ⚙️ Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `wallet_address` | Your Solana wallet public key | Required |
| `private_key` | Your wallet private key (base58) | Required |
| `rpc_url` | Solana RPC endpoint | mainnet-beta |
| `initial_balance_usd` | Starting USD balance | 10.0 |
| `price_check_interval_seconds` | Price check frequency | 10 |
| `swap_threshold_min_percent` | Minimum price change | 3.0% |
| `swap_threshold_max_percent` | Maximum price change | 5.0% |
| `max_swaps_per_day` | Daily swap limit | 3 |
| `slippage_bps` | Slippage tolerance (basis points) | 50 (0.5%) |
| `simulate_mode` | Enable simulation mode | true |
| `priority_fee_microlamports` | Transaction priority fee | 10000 |

## 🔥 Real Trading Features

### ✅ Complete Implementation
- **Jupiter API Integration**: Quote + Swap APIs
- **Solana Transaction Signing**: Direct wallet integration
- **Transaction Confirmation**: Monitors tx status until confirmed
- **Error Handling**: Comprehensive error recovery
- **Balance Tracking**: Real-time asset tracking
- **Priority Fees**: Configurable transaction fees

### 🛡️ Safety Features
- **Simulation Mode**: Test without real funds
- **Daily Limits**: Prevents overtrading
- **Block Height Validation**: Prevents stale transactions
- **Slippage Protection**: Configurable slippage limits
- **Transaction Timeouts**: Prevents stuck transactions

## 📝 Example Output

```
Starting Solana Trading Bot
Initial balance: $10.00 in SOL
Wallet address: 7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRueJD
Price check interval: 10s
Swap thresholds: 3.0% - 5.0%
Max swaps per day: 3
Simulate mode: true

Current SOL price: $195.37 | Holding: SOL ($10.00) | Last swap: $0.00
Swap decision: Initial price set

Current SOL price: $201.24 | Holding: SOL ($10.00) | Last swap: $195.37
Swap decision: SOL price increased by 3.01% (195.37 -> 201.24), swapping to USDC
EXECUTING REAL SWAP: SOL -> USDC
Transaction sent: 4xK2...abc123
Transaction confirmed with status: confirmed
New balance: $10.30 in USDC
Swaps today: 1/3
```

## 🚨 Risk Disclaimer

**⚠️ HIGH RISK TRADING BOT**

- **Real Money**: This bot trades real cryptocurrency
- **Market Risk**: Crypto prices are highly volatile
- **Smart Contract Risk**: Jupiter protocol risks
- **Bug Risk**: Software may have bugs
- **Loss Risk**: You may lose all invested funds

**Start small and test thoroughly!**

## 🔧 Advanced Usage

### Custom RPC Endpoints
```json
"rpc_url": "https://solana-mainnet.g.alchemy.com/v2/YOUR-API-KEY"
```

### Devnet Testing
```json
"rpc_url": "https://api.devnet.solana.com"
```

### Priority Fee Optimization
```json
"priority_fee_microlamports": 50000  // Higher fees = faster execution
```

## 📊 Monitoring

The bot logs:
- Price movements and decisions
- Swap executions and confirmations
- Balance changes and profits
- Error conditions and recoveries

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch
3. Test thoroughly with simulation mode
4. Submit a pull request

## 📄 License

MIT License - Use at your own risk!