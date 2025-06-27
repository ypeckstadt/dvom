package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// SaltSize is the size of the salt for key derivation
	SaltSize = 32
	// KeySize is the size of the AES key (256 bits)
	KeySize = 32
	// NonceSize is the size of the GCM nonce
	NonceSize = 12
	// Iterations for PBKDF2
	Iterations = 100000
)

// EncryptionHeader contains encryption metadata
type EncryptionHeader struct {
	Salt  []byte
	Nonce []byte
}

// DeriveKey derives an encryption key from a password using PBKDF2
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, Iterations, KeySize, sha256.New)
}

// GenerateSalt generates a random salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GenerateNonce generates a random nonce for GCM
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return nonce, nil
}

// EncryptReader wraps a reader with AES-256-GCM encryption
type EncryptReader struct {
	reader    io.Reader
	cipher    cipher.AEAD
	baseNonce []byte
	counter   uint64
	buffer    []byte
	encrypted []byte
	eof       bool
}

// NewEncryptReader creates a new encrypting reader
func NewEncryptReader(r io.Reader, password string) (*EncryptReader, *EncryptionHeader, error) {
	// Generate salt and derive key
	salt, err := GenerateSalt()
	if err != nil {
		return nil, nil, err
	}
	
	key := DeriveKey(password, salt)
	
	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Generate nonce
	nonce, err := GenerateNonce()
	if err != nil {
		return nil, nil, err
	}
	
	header := &EncryptionHeader{
		Salt:  salt,
		Nonce: nonce,
	}
	
	return &EncryptReader{
		reader:    r,
		cipher:    gcm,
		baseNonce: nonce,
		counter:   0,
		buffer:    make([]byte, 64*1024), // 64KB chunks
	}, header, nil
}

// Read implements io.Reader with encryption
func (er *EncryptReader) Read(p []byte) (int, error) {
	if er.eof && len(er.encrypted) == 0 {
		return 0, io.EOF
	}
	
	// If we have encrypted data, return it
	if len(er.encrypted) > 0 {
		n := copy(p, er.encrypted)
		er.encrypted = er.encrypted[n:]
		return n, nil
	}
	
	// Read more data
	n, err := er.reader.Read(er.buffer)
	if err != nil && err != io.EOF {
		return 0, err
	}
	
	if n > 0 {
		// Create unique nonce for this chunk
		chunkNonce := make([]byte, len(er.baseNonce))
		copy(chunkNonce, er.baseNonce)
		
		// Combine base nonce with counter to ensure uniqueness
		for i := 0; i < 8 && i < len(chunkNonce); i++ {
			chunkNonce[len(chunkNonce)-1-i] ^= byte(er.counter >> (8 * i))
		}
		
		// Encrypt the chunk
		er.encrypted = er.cipher.Seal(nil, chunkNonce, er.buffer[:n], nil)
		er.counter++
		
		// Copy to output
		copied := copy(p, er.encrypted)
		er.encrypted = er.encrypted[copied:]
		return copied, nil
	}
	
	if err == io.EOF {
		er.eof = true
		if len(er.encrypted) > 0 {
			n := copy(p, er.encrypted)
			er.encrypted = er.encrypted[n:]
			return n, nil
		}
		return 0, io.EOF
	}
	
	return 0, nil
}

// DecryptReader wraps a reader with AES-256-GCM decryption
type DecryptReader struct {
	reader    io.Reader
	cipher    cipher.AEAD
	baseNonce []byte
	counter   uint64
	buffer    []byte
	decrypted []byte
	eof       bool
}

// NewDecryptReader creates a new decrypting reader
func NewDecryptReader(r io.Reader, password string, header *EncryptionHeader) (*DecryptReader, error) {
	// Derive key from password and salt
	key := DeriveKey(password, header.Salt)
	
	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Copy nonce to avoid modifying the header
	baseNonce := make([]byte, len(header.Nonce))
	copy(baseNonce, header.Nonce)
	
	return &DecryptReader{
		reader:    r,
		cipher:    gcm,
		baseNonce: baseNonce,
		counter:   0,
		buffer:    make([]byte, 64*1024+gcm.Overhead()), // 64KB + overhead
	}, nil
}

// Read implements io.Reader with decryption
func (dr *DecryptReader) Read(p []byte) (int, error) {
	if dr.eof && len(dr.decrypted) == 0 {
		return 0, io.EOF
	}
	
	// If we have decrypted data, return it
	if len(dr.decrypted) > 0 {
		n := copy(p, dr.decrypted)
		dr.decrypted = dr.decrypted[n:]
		return n, nil
	}
	
	// Read more data (encrypted chunk size)
	n, err := dr.reader.Read(dr.buffer)
	if err != nil && err != io.EOF {
		return 0, err
	}
	
	if n > 0 {
		// Create unique nonce for this chunk (same logic as encryption)
		chunkNonce := make([]byte, len(dr.baseNonce))
		copy(chunkNonce, dr.baseNonce)
		
		// Combine base nonce with counter to ensure uniqueness
		for i := 0; i < 8 && i < len(chunkNonce); i++ {
			chunkNonce[len(chunkNonce)-1-i] ^= byte(dr.counter >> (8 * i))
		}
		
		// Decrypt the chunk
		decrypted, err := dr.cipher.Open(nil, chunkNonce, dr.buffer[:n], nil)
		if err != nil {
			return 0, fmt.Errorf("decryption failed: %w", err)
		}
		dr.decrypted = decrypted
		dr.counter++
		
		// Copy to output
		copied := copy(p, dr.decrypted)
		dr.decrypted = dr.decrypted[copied:]
		return copied, nil
	}
	
	if err == io.EOF {
		dr.eof = true
		if len(dr.decrypted) > 0 {
			n := copy(p, dr.decrypted)
			dr.decrypted = dr.decrypted[n:]
			return n, nil
		}
		return 0, io.EOF
	}
	
	return 0, nil
}


// WriteEncryptionHeader writes the encryption header to a writer
func WriteEncryptionHeader(w io.Writer, header *EncryptionHeader) error {
	// Write magic bytes "DVOM-ENC" to identify encrypted backups
	if _, err := w.Write([]byte("DVOM-ENC")); err != nil {
		return fmt.Errorf("failed to write magic bytes: %w", err)
	}
	
	// Write version byte (1)
	if _, err := w.Write([]byte{1}); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	
	// Write salt
	if _, err := w.Write(header.Salt); err != nil {
		return fmt.Errorf("failed to write salt: %w", err)
	}
	
	// Write nonce
	if _, err := w.Write(header.Nonce); err != nil {
		return fmt.Errorf("failed to write nonce: %w", err)
	}
	
	return nil
}

// ReadEncryptionHeader reads the encryption header from a reader
func ReadEncryptionHeader(r io.Reader) (*EncryptionHeader, error) {
	// Read magic bytes
	magic := make([]byte, 8)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("failed to read magic bytes: %w", err)
	}
	
	if string(magic) != "DVOM-ENC" {
		return nil, fmt.Errorf("not an encrypted DVOM backup")
	}
	
	// Read version
	version := make([]byte, 1)
	if _, err := io.ReadFull(r, version); err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	
	if version[0] != 1 {
		return nil, fmt.Errorf("unsupported encryption version: %d", version[0])
	}
	
	// Read salt
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(r, salt); err != nil {
		return nil, fmt.Errorf("failed to read salt: %w", err)
	}
	
	// Read nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(r, nonce); err != nil {
		return nil, fmt.Errorf("failed to read nonce: %w", err)
	}
	
	return &EncryptionHeader{
		Salt:  salt,
		Nonce: nonce,
	}, nil
}

// IsEncrypted checks if data starts with encryption header
func IsEncrypted(data []byte) bool {
	return len(data) >= 8 && string(data[:8]) == "DVOM-ENC"
}