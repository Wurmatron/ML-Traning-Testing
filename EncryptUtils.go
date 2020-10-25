package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/scrypt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var encryptionPass string

// https://bruinsslot.jp/post/golang-crypto/
// Jan Pieter
func Encrypt(key, data []byte) ([]byte, error) {
	key, salt, err := DeriveKey(key, nil)
	if err != nil {
		return nil, err
	}

	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	ciphertext = append(ciphertext, salt...)

	return ciphertext, nil
}

// https://bruinsslot.jp/post/golang-crypto/
// Jan Pieter
func Decrypt(key, data []byte) ([]byte, error) {
	salt, data := data[len(data)-32:], data[:len(data)-32]

	key, _, err := DeriveKey(key, salt)
	if err != nil {
		return nil, err
	}

	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return nil, err
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// https://bruinsslot.jp/post/golang-crypto/
// Jan Pieter
func DeriveKey(password, salt []byte) ([]byte, []byte, error) {
	if salt == nil {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, nil, err
		}
	}
	key, err := scrypt.Key(password, salt, 1048576, 8, 1, 32)
	if err != nil {
		return nil, nil, err
	}

	return key, salt, nil
}

func setupOrLoadEncryptionKey() {
	var encryptionDir = BaseDir + "/encryption/"
	_, err := os.Stat(encryptionDir)
	if os.IsNotExist(err) {
		err2 := os.Mkdir(encryptionDir, 0755)
		if err2 != nil {
			log.Fatal(err)
		}
	}
	_, err = os.Stat(encryptionDir + "passwd.txt")
	if os.IsNotExist(err) {
		for {
			fmt.Println("This password is non-recoverable, Its used to encrypt files")
			password := readPassword("Enter a password to encrypt the Token's with: ")
			password2 := readPassword("Re-Enter the password: ")
			if password == password2 && len(password) > 0 {
				encryptionPass = password
				WriteToFile(encryptionDir+"passwd.txt", HashString(encryptionPass))
				return
			}
		}
	} else { // Already created
		for {
			passwordHash := ReadFile(encryptionDir + "passwd.txt")
			password := readPassword("Enter the encryption password: ")
			err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
			if err == nil {
				encryptionPass = password
				return
			} else {
				fmt.Println(err)
				fmt.Println("Invalid Encryption Passcode! ")
			}
		}
	}
}

func readPassword(prompt string) string {
	fmt.Print(prompt)
	var line string
	fmt.Scanln(&line)
	return line
}

func HashString(data string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(data), bcrypt.DefaultCost)
	return string(hash)
}

func WriteToFile(filename string, data string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.WriteString(file, data)
	if err != nil {
		return err
	}
	return file.Sync()
}

func ReadFile(filename string) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("File reading error", err)
		return ""
	}
	return string(data)
}
