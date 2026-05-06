package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
)

// Cek apakah prefix valid dan tentukan apakah ini SegWit
func isPrefixValid(prefix string) (bool, bool) {
	isSegwit := false

	if strings.HasPrefix(prefix, "1") {
		// Validasi Legacy (Base58Check)
		invalidChars := "0OIl"
		for _, char := range prefix {
			if strings.ContainsRune(invalidChars, char) {
				fmt.Printf("Error: Karakter '%c' tidak diperbolehkan dalam Base58 Bitcoin.\n", char)
				return false, isSegwit
			}
		}
	} else if strings.HasPrefix(prefix, "bc1q") {
		// Validasi Native SegWit (Bech32)
		isSegwit = true
		// Karakter valid Bech32 (tanpa 1, b, i, o)
		validBech32Chars := "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

		// Cek karakter setelah "bc1q"
		for _, char := range prefix[4:] {
			if !strings.ContainsRune(validBech32Chars, char) {
				fmt.Printf("Error: Karakter '%c' tidak diperbolehkan dalam Bech32 SegWit.\nKarakter yang diizinkan: %s\n", char, validBech32Chars)
				return false, isSegwit
			}
		}
	} else {
		fmt.Println("Error: Prefix harus dimulai dengan '1' (Legacy) atau 'bc1q' (Native SegWit).")
		return false, isSegwit
	}

	return true, isSegwit
}

// Fungsi worker untuk mencari vanity address
func worker(prefix string, isSegwit bool, resultChan chan<- string, stopChan <-chan struct{}, wg *sync.WaitGroup, counter *uint64) {
	defer wg.Done()

	for {
		// Cek apakah proses harus dihentikan
		select {
		case <-stopChan:
			return
		default:
			// Lanjut mencari
		}

		// Tambah counter secara aman untuk goroutine (thread-safe)
		atomic.AddUint64(counter, 1)

		// 1. Buat Private Key baru (secp256k1)
		privKey, err := btcec.NewPrivateKey()
		if err != nil {
			continue
		}

		// 2. Dapatkan Public Key Hash (RIPEMD160(SHA256(PubKey)))
		pubKeyHash := btcutil.Hash160(privKey.PubKey().SerializeCompressed())

		// 3. Buat alamat sesuai tipe (Legacy atau SegWit) untuk Mainnet
		var addr btcutil.Address
		if isSegwit {
			addr, err = btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, &chaincfg.MainNetParams)
		} else {
			addr, err = btcutil.NewAddressPubKeyHash(pubKeyHash, &chaincfg.MainNetParams)
		}

		if err != nil {
			continue
		}

		addrStr := addr.EncodeAddress()

		// 4. Cek apakah alamat cocok dengan prefix yang dicari
		if strings.HasPrefix(addrStr, prefix) {
			// Jika cocok, buat format WIF (Wallet Import Format) untuk private key
			wif, _ := btcutil.NewWIF(privKey, &chaincfg.MainNetParams, true)

			result := fmt.Sprintf("\n🎉 BERHASIL DITEMUKAN! 🎉\nAlamat Bitcoin  : %s\nPrivate Key WIF : %s\n", addrStr, wif.String())

			select {
			case resultChan <- result:
			case <-stopChan:
			}
			return
		}
	}
}

func main() {
	// Cek apakah argumen prefix diberikan saat menjalankan program
	if len(os.Args) < 2 {
		fmt.Println("Cara penggunaan: go run . <prefix>")
		fmt.Println("Contoh Legacy  : go run . 1Go")
		fmt.Println("Contoh SegWit  : go run . bc1qbot")
		os.Exit(1)
	}

	// Mengambil prefix dari argumen pertama
	targetPrefix := strings.ToLower(os.Args[1]) // Ubah ke huruf kecil khusus untuk validasi awal jika segwit
	if strings.HasPrefix(os.Args[1], "1") {
		targetPrefix = os.Args[1] // Legacy tetap *case-sensitive*
	}

	isValid, isSegwit := isPrefixValid(targetPrefix)
	if !isValid {
		os.Exit(1)
	}

	tipeAlamat := "Legacy (P2PKH)"
	if isSegwit {
		tipeAlamat = "Native SegWit (P2WPKH)"
	}

	fmt.Println("========================================")
	fmt.Println("  Mencari Vanity Bitcoin Address...")
	fmt.Println("  Target Prefix :", targetPrefix)
	fmt.Println("  Tipe Alamat   :", tipeAlamat)
	fmt.Println("========================================")

	// Gunakan semua core CPU yang tersedia untuk mempercepat pencarian
	numCores := runtime.NumCPU()
	runtime.GOMAXPROCS(numCores)
	fmt.Printf("Menggunakan %d CPU cores untuk pencarian...\n\n", numCores)

	resultChan := make(chan string)
	stopChan := make(chan struct{})
	var wg sync.WaitGroup
	var counter uint64

	// Jalankan workers sesuai jumlah core CPU
	for i := 0; i < numCores; i++ {
		wg.Add(1)
		go worker(targetPrefix, isSegwit, resultChan, stopChan, &wg, &counter)
	}

	// Timer untuk menghitung lama pencarian
	startTime := time.Now()

	// Ticker untuk menampilkan progress setiap 1 detik
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Loop untuk memonitor hasil atau progress
	for {
		select {
		case result := <-resultChan:
			// Jika berhasil, hentikan worker dan tampilkan hasil
			close(stopChan)
			fmt.Println(result)
			fmt.Printf("Waktu pencarian: %s\n", time.Since(startTime))
			fmt.Println("⚠️ PERINGATAN: Simpan Private Key Anda di tempat yang aman!")
			wg.Wait()
			return

		case <-ticker.C:
			// Tampilkan status progress setiap detiknya
			ops := atomic.LoadUint64(&counter)
			elapsed := time.Since(startTime).Seconds()
			rate := float64(ops) / elapsed
			fmt.Printf("\r⏳ Diperiksa: %d alamat (Kecepatan: %.0f kunci/detik)...", ops, rate)
		}
	}
}
