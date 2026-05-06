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

// Cek apakah prefix valid untuk Base58Check Bitcoin
func isPrefixValid(prefix string) bool {
	// Alamat Legacy (P2PKH) Mainnet harus dimulai dengan '1'
	if !strings.HasPrefix(prefix, "1") {
		fmt.Println("Error: Alamat Bitcoin Legacy harus dimulai dengan angka '1'.")
		return false
	}

	// Karakter yang tidak diperbolehkan dalam Base58 (0, O, I, l)
	invalidChars := "0OIl"
	for _, char := range prefix {
		if strings.ContainsRune(invalidChars, char) {
			fmt.Printf("Error: Karakter '%c' tidak diperbolehkan dalam Base58 Bitcoin.\n", char)
			return false
		}
	}

	return true
}

// Fungsi worker untuk mencari vanity address
func worker(prefix string, resultChan chan<- string, stopChan <-chan struct{}, wg *sync.WaitGroup, counter *uint64) {
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

		// 3. Buat alamat P2PKH (Pay-to-PubKey-Hash) untuk Mainnet
		addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, &chaincfg.MainNetParams)
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
		fmt.Println("Contoh: go run . 1Go")
		os.Exit(1)
	}

	// Mengambil prefix dari argumen pertama
	targetPrefix := os.Args[1]

	fmt.Println("========================================")
	fmt.Println("  Mencari Vanity Bitcoin Address...")
	fmt.Println("  Target Prefix :", targetPrefix)
	fmt.Println("========================================")

	if !isPrefixValid(targetPrefix) {
		os.Exit(1)
	}

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
		go worker(targetPrefix, resultChan, stopChan, &wg, &counter)
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
