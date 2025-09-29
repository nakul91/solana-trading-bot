package main

import (
	"crypto/ed25519"
	"fmt"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

func testDerivation() {
	mnemonic := "stove another another movie noise assume divorce cloud artefact caught elephant end"

	// Validate mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		log.Fatal("Invalid mnemonic")
	}

	// Generate seed from mnemonic
	seed := bip39.NewSeed(mnemonic, "")
	fmt.Printf("Seed length: %d\n", len(seed))

	// Method 1: Direct from seed (first 32 bytes)
	privateKey1 := ed25519.NewKeyFromSeed(seed[:32])
	solanaKey1 := solana.PrivateKey(privateKey1)
	fmt.Printf("Method 1 (direct): %s\n", solanaKey1.PublicKey().String())

	// Method 2: BIP44 derivation m/44'/501'/0'/0'
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		log.Fatal(err)
	}

	purpose, _ := masterKey.NewChildKey(0x8000002C)  // 44'
	coinType, _ := purpose.NewChildKey(0x800001F5)  // 501'
	account, _ := coinType.NewChildKey(0x80000000)  // 0'
	change, _ := account.NewChildKey(0x80000000)    // 0'

	privateKey2 := ed25519.NewKeyFromSeed(change.Key)
	solanaKey2 := solana.PrivateKey(privateKey2)
	fmt.Printf("Method 2 (BIP44 m/44'/501'/0'/0'): %s\n", solanaKey2.PublicKey().String())

	// Method 3: Different derivation m/44'/501'/0'
	account2, _ := coinType.NewChildKey(0x80000000)  // 0'
	privateKey3 := ed25519.NewKeyFromSeed(account2.Key)
	solanaKey3 := solana.PrivateKey(privateKey3)
	fmt.Printf("Method 3 (BIP44 m/44'/501'/0'): %s\n", solanaKey3.PublicKey().String())

	// Method 4: Phantom wallet style (common alternative)
	change2, _ := account.NewChildKey(0x00000000)    // 0 (non-hardened)
	privateKey4 := ed25519.NewKeyFromSeed(change2.Key)
	solanaKey4 := solana.PrivateKey(privateKey4)
	fmt.Printf("Method 4 (m/44'/501'/0'/0): %s\n", solanaKey4.PublicKey().String())
}

func main() {
	testDerivation()
}