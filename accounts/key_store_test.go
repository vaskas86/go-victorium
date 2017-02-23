// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package accounts

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereumproject/go-ethereum/common"
	"github.com/ethereumproject/go-ethereum/crypto"
	"github.com/ethereumproject/go-ethereum/crypto/secp256k1"
)

func tmpKeyStore(t *testing.T, encrypted bool) (dir string, ks keyStore) {
	d, err := ioutil.TempDir("", "geth-keystore-test")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted {
		ks = &keyStorePassphrase{d, veryLightScryptN, veryLightScryptP}
	} else {
		ks = &keyStorePlain{d}
	}
	return d, ks
}

func TestKeyStorePlain(t *testing.T) {
	dir, ks := tmpKeyStore(t, false)
	defer os.RemoveAll(dir)

	pass := "" // not used but required by API
	k1, account, err := storeNewKey(ks, rand.Reader, pass)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ks.GetKey(k1.Address, account.File, pass)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.Address, k2.Address) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.PrivateKey, k2.PrivateKey) {
		t.Fatal(err)
	}
}

func TestKeyStorePassphrase(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	pass := "foo"
	k1, account, err := storeNewKey(ks, rand.Reader, pass)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ks.GetKey(k1.Address, account.File, pass)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.Address, k2.Address) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.PrivateKey, k2.PrivateKey) {
		t.Fatal(err)
	}
}

func TestKeyStorePassphraseDecryptionFail(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	pass := "foo"
	k1, account, err := storeNewKey(ks, rand.Reader, pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ks.GetKey(k1.Address, account.File, "bar"); err != ErrDecrypt {
		t.Fatalf("wrong error for invalid passphrase\ngot %q\nwant %q", err, ErrDecrypt)
	}
}

func TestImportPreSaleKey(t *testing.T) {
	dir, ks := tmpKeyStore(t, true)
	defer os.RemoveAll(dir)

	// file content of a presale key file generated with:
	// python pyethsaletool.py genwallet
	// with password "foo"
	fileContent := "{\"encseed\": \"26d87f5f2bf9835f9a47eefae571bc09f9107bb13d54ff12a4ec095d01f83897494cf34f7bed2ed34126ecba9db7b62de56c9d7cd136520a0427bfb11b8954ba7ac39b90d4650d3448e31185affcd74226a68f1e94b1108e6e0a4a91cdd83eba\", \"ethaddr\": \"d4584b5f6229b7be90727b0fc8c6b91bb427821f\", \"email\": \"gustav.simonsson@gmail.com\", \"btcaddr\": \"1EVknXyFC68kKNLkh6YnKzW41svSRoaAcx\"}"
	pass := "foo"
	account, _, err := importPreSaleKey(ks, []byte(fileContent), pass)
	if err != nil {
		t.Fatal(err)
	}
	if account.Address != common.HexToAddress("d4584b5f6229b7be90727b0fc8c6b91bb427821f") {
		t.Errorf("imported account has wrong address %x", account.Address)
	}
	if !strings.HasPrefix(account.File, dir) {
		t.Errorf("imported account file not in keystore directory: %q", account.File)
	}
}

// Test and utils for the key store tests in the Ethereum JSON tests;
// testdataKeyStoreTests/basic_tests.json
type KeyStoreTestV3 struct {
	Json     encryptedKeyJSONV3
	Password string
	Priv     string
}

type KeyStoreTestV1 struct {
	Json     encryptedKeyJSONV1
	Password string
	Priv     string
}

func TestV3_PBKDF2_1(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("testdata/v3_test_vector.json", t)
	testDecryptV3(tests["wikipage_test_vector_pbkdf2"], t)
}

func TestV3_PBKDF2_2(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("../tests/files/KeyStoreTests/basic_tests.json", t)
	testDecryptV3(tests["test1"], t)
}

func TestV3_PBKDF2_3(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("../tests/files/KeyStoreTests/basic_tests.json", t)
	testDecryptV3(tests["python_generated_test_with_odd_iv"], t)
}

func TestV3_PBKDF2_4(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("../tests/files/KeyStoreTests/basic_tests.json", t)
	testDecryptV3(tests["evilnonce"], t)
}

func TestV3_Scrypt_1(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("testdata/v3_test_vector.json", t)
	testDecryptV3(tests["wikipage_test_vector_scrypt"], t)
}

func TestV3_Scrypt_2(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV3("../tests/files/KeyStoreTests/basic_tests.json", t)
	testDecryptV3(tests["test2"], t)
}

func TestV1_1(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV1("testdata/v1_test_vector.json", t)
	testDecryptV1(tests["test1"], t)
}

func TestV1_2(t *testing.T) {
	t.Parallel()
	ks := &keyStorePassphrase{"testdata/v1", LightScryptN, LightScryptP}
	addr := common.HexToAddress("cb61d5a9c4896fb9658090b597ef0e7be6f7b67e")
	file := "testdata/v1/cb61d5a9c4896fb9658090b597ef0e7be6f7b67e/cb61d5a9c4896fb9658090b597ef0e7be6f7b67e"
	k, err := ks.GetKey(addr, file, "g")
	if err != nil {
		t.Fatal(err)
	}
	privHex := hex.EncodeToString(crypto.FromECDSA(k.PrivateKey))
	expectedHex := "d1b1178d3529626a1a93e073f65028370d14c7eb0936eb42abef05db6f37ad7d"
	if privHex != expectedHex {
		t.Fatal(fmt.Errorf("Unexpected privkey: %v, expected %v", privHex, expectedHex))
	}
}

func testDecryptV3(test KeyStoreTestV3, t *testing.T) {
	privBytes, _, err := decryptKeyV3(&test.Json, test.Password)
	if err != nil {
		t.Fatal(err)
	}
	privHex := hex.EncodeToString(privBytes)
	if test.Priv != privHex {
		t.Fatal(fmt.Errorf("Decrypted bytes not equal to test, expected %v have %v", test.Priv, privHex))
	}
}

func testDecryptV1(test KeyStoreTestV1, t *testing.T) {
	privBytes, _, err := decryptKeyV1(&test.Json, test.Password)
	if err != nil {
		t.Fatal(err)
	}
	privHex := hex.EncodeToString(privBytes)
	if test.Priv != privHex {
		t.Fatal(fmt.Errorf("Decrypted bytes not equal to test, expected %v have %v", test.Priv, privHex))
	}
}

func loadKeyStoreTestV3(file string, t *testing.T) map[string]KeyStoreTestV3 {
	tests := make(map[string]KeyStoreTestV3)
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(bytes, &tests); err != nil {
		t.Fatal(err)
	}
	return tests
}

func loadKeyStoreTestV1(file string, t *testing.T) map[string]KeyStoreTestV1 {
	tests := make(map[string]KeyStoreTestV1)
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(bytes, &tests); err != nil {
		t.Fatal(err)
	}
	return tests
}

func TestKeyForDirectICAP(t *testing.T) {
	t.Parallel()
	key := newKeyForDirectICAP(rand.Reader)
	if !strings.HasPrefix(key.Address.Hex(), "0x00") {
		t.Errorf("Expected first address byte to be zero, have: %s", key.Address.Hex())
	}
}

// newKeyForDirectICAP generates a key whose address fits into < 155 bits so it can fit
// into the Direct ICAP spec. for simplicity and easier compatibility with other libs, we
// retry until the first byte is 0.
func newKeyForDirectICAP(rand io.Reader) *Key {
	randBytes := make([]byte, 64)
	_, err := rand.Read(randBytes)
	if err != nil {
		panic("key generation: could not read from random source: " + err.Error())
	}
	reader := bytes.NewReader(randBytes)
	privateKeyECDSA, err := ecdsa.GenerateKey(secp256k1.S256(), reader)
	if err != nil {
		panic("key generation: ecdsa.GenerateKey failed: " + err.Error())
	}
	key := newKeyFromECDSA(privateKeyECDSA)
	if !strings.HasPrefix(key.Address.Hex(), "0x00") {
		return newKeyForDirectICAP(rand)
	}
	return key
}