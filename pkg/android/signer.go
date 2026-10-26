// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	oidData          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}
	oidSignedData    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidSHA256        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidRSAEncryption = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}

	// APK Signature Scheme v2 constants
	apkSigSchemeV2BlockID = uint32(0x7109871a)
	apkSigBlockMagic      = []byte("APK Sig Block 42") // 16 bytes
	sigAlgoRSASHA256      = uint32(0x0103)             // SIGNATURE_RSA_PKCS1_V1_5_WITH_SHA256
)

type pkcs7ContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,tag:0"`
}

type pkcs7SignedData struct {
	Version                    int
	DigestAlgorithmIdentifiers []pkcs7AlgorithmIdentifier `asn1:"set"`
	ContentInfo                pkcs7EncapsulatedContentInfo
	Certificates               asn1.RawValue     `asn1:"optional,tag:0"`
	SignerInfos                []pkcs7SignerInfo `asn1:"set"`
}

type pkcs7EncapsulatedContentInfo struct {
	ContentType asn1.ObjectIdentifier
}

type pkcs7AlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type pkcs7SignerInfo struct {
	Version                   int
	IssuerAndSerialNumber     pkcs7IssuerAndSerialNumber
	DigestAlgorithm           pkcs7AlgorithmIdentifier
	DigestEncryptionAlgorithm pkcs7AlgorithmIdentifier
	EncryptedDigest           []byte
}

type pkcs7IssuerAndSerialNumber struct {
	IssuerName   asn1.RawValue
	SerialNumber *big.Int
}

type persistentKeystore struct {
	KeyDERB64  string `json:"key"`
	CertDERB64 string `json:"cert"`
}

var (
	cachedKeyMu   sync.Mutex
	cachedKey     *rsa.PrivateKey
	cachedCert    *x509.Certificate
	cachedCertDER []byte
)

func getOrCreateDebugKey() (*rsa.PrivateKey, *x509.Certificate, []byte, error) {
	cachedKeyMu.Lock()
	defer cachedKeyMu.Unlock()

	if cachedKey != nil {
		return cachedKey, cachedCert, cachedCertDER, nil
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		alcaDir := filepath.Join(homeDir, ".alca")
		keystoreFile := filepath.Join(alcaDir, "debug.keystore.json")
		if data, err := os.ReadFile(keystoreFile); err == nil {
			var ks persistentKeystore
			if err := json.Unmarshal(data, &ks); err == nil {
				keyBytes, err1 := base64.StdEncoding.DecodeString(ks.KeyDERB64)
				certBytes, err2 := base64.StdEncoding.DecodeString(ks.CertDERB64)
				if err1 == nil && err2 == nil {
					key, errKey := x509.ParsePKCS1PrivateKey(keyBytes)
					cert, errCert := x509.ParseCertificate(certBytes)
					if errKey == nil && errCert == nil {
						cachedKey = key
						cachedCert = cert
						cachedCertDER = certBytes
						return key, cert, certBytes, nil
					}
				}
			}
		}
	}

	// Generate persistent key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generating RSA debug key: %w", err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "Android Debug",
			Organization: []string{"Android"},
			Country:      []string{"US"},
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(30 * 365 * 24 * time.Hour), // 30 years
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating debug cert: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing debug cert: %w", err)
	}

	// Save to disk if possible
	if homeDir != "" {
		alcaDir := filepath.Join(homeDir, ".alca")
		_ = os.MkdirAll(alcaDir, 0o755)
		keystoreFile := filepath.Join(alcaDir, "debug.keystore.json")
		ks := persistentKeystore{
			KeyDERB64:  base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(key)),
			CertDERB64: base64.StdEncoding.EncodeToString(certDER),
		}
		if data, err := json.Marshal(ks); err == nil {
			_ = os.WriteFile(keystoreFile, data, 0o600)
		}
	}

	cachedKey = key
	cachedCert = cert
	cachedCertDER = certDER

	return key, cert, certDER, nil
}

// SignAPK creates standard Android v1 JAR signature files (MANIFEST.MF, CERT.SF, CERT.RSA).
func SignAPK(entries map[string][]byte) (manifestMF, certSF, certRSA []byte, err error) {
	key, cert, certDER, err := getOrCreateDebugKey()
	if err != nil {
		return nil, nil, nil, err
	}

	// Deterministic alphabetical sorting of entries
	var names []string
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	// 1. Build META-INF/MANIFEST.MF
	var mfBuf bytes.Buffer
	mfBuf.WriteString("Manifest-Version: 1.0\r\n")
	mfBuf.WriteString("Created-By: 1.0 (Alca Mobile Builder)\r\n\r\n")

	entryHeaders := make(map[string]string)

	for _, name := range names {
		data := entries[name]
		h := sha256.Sum256(data)
		digestB64 := base64.StdEncoding.EncodeToString(h[:])

		entryHeader := fmt.Sprintf("Name: %s\r\nSHA-256-Digest: %s\r\n", name, digestB64)
		mfBuf.WriteString(entryHeader + "\r\n")
		entryHeaders[name] = entryHeader
	}
	manifestMF = mfBuf.Bytes()

	// 2. Build META-INF/CERT.SF
	var sfBuf bytes.Buffer
	sfBuf.WriteString("Signature-Version: 1.0\r\n")
	sfBuf.WriteString("Created-By: 1.0 (Alca Mobile Builder)\r\n")

	mfDigest := sha256.Sum256(manifestMF)
	sfBuf.WriteString(fmt.Sprintf("SHA-256-Digest-Manifest: %s\r\n\r\n", base64.StdEncoding.EncodeToString(mfDigest[:])))

	for _, name := range names {
		headerText := entryHeaders[name]
		secDigest := sha256.Sum256([]byte(headerText))
		sfBuf.WriteString(fmt.Sprintf("Name: %s\r\nSHA-256-Digest: %s\r\n\r\n", name, base64.StdEncoding.EncodeToString(secDigest[:])))
	}
	certSF = sfBuf.Bytes()

	// 3. Build META-INF/CERT.RSA (PKCS#7 SignedData)
	sfHashed := sha256.Sum256(certSF)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sfHashed[:])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("signing CERT.SF: %w", err)
	}

	signerInfo := pkcs7SignerInfo{
		Version: 1,
		IssuerAndSerialNumber: pkcs7IssuerAndSerialNumber{
			IssuerName:   asn1.RawValue{FullBytes: cert.RawIssuer},
			SerialNumber: cert.SerialNumber,
		},
		DigestAlgorithm: pkcs7AlgorithmIdentifier{
			Algorithm:  oidSHA256,
			Parameters: asn1.RawValue{Tag: 5}, // NULL
		},
		DigestEncryptionAlgorithm: pkcs7AlgorithmIdentifier{
			Algorithm:  oidRSAEncryption,
			Parameters: asn1.RawValue{Tag: 5}, // NULL
		},
		EncryptedDigest: sig,
	}

	sd := pkcs7SignedData{
		Version: 1,
		DigestAlgorithmIdentifiers: []pkcs7AlgorithmIdentifier{
			{
				Algorithm:  oidSHA256,
				Parameters: asn1.RawValue{Tag: 5}, // NULL
			},
		},
		ContentInfo: pkcs7EncapsulatedContentInfo{
			ContentType: oidData,
		},
		Certificates: asn1.RawValue{
			Class:      2, // Context-specific
			Tag:        0,
			IsCompound: true,
			Bytes:      certDER,
		},
		SignerInfos: []pkcs7SignerInfo{signerInfo},
	}

	sdBytes, err := asn1.Marshal(sd)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshaling SignedData: %w", err)
	}

	contentInfo := pkcs7ContentInfo{
		ContentType: oidSignedData,
		Content: asn1.RawValue{
			Class:      2,
			Tag:        0,
			IsCompound: true,
			Bytes:      sdBytes,
		},
	}

	certRSA, err = asn1.Marshal(contentInfo)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshaling ContentInfo: %w", err)
	}

	return manifestMF, certSF, certRSA, nil
}

// ApplyAPKSignatureSchemeV2 signs an aligned ZIP file with APK Signature Scheme v2.
func ApplyAPKSignatureSchemeV2(zipBytes []byte) ([]byte, error) {
	key, cert, _, err := getOrCreateDebugKey()
	if err != nil {
		return nil, err
	}

	// 1. Find EOCD (End of Central Directory)
	if len(zipBytes) < 22 {
		return nil, fmt.Errorf("ZIP archive too small")
	}

	eocdOffset := -1
	for i := len(zipBytes) - 22; i >= 0 && i >= len(zipBytes)-65557; i-- {
		if binary.LittleEndian.Uint32(zipBytes[i:i+4]) == 0x06054b50 {
			eocdOffset = i
			break
		}
	}
	if eocdOffset == -1 {
		return nil, fmt.Errorf("EOCD record not found in ZIP archive")
	}

	cdOffset := binary.LittleEndian.Uint32(zipBytes[eocdOffset+16 : eocdOffset+20])
	cdSize := binary.LittleEndian.Uint32(zipBytes[eocdOffset+12 : eocdOffset+16])

	if int(cdOffset)+int(cdSize) > eocdOffset {
		return nil, fmt.Errorf("invalid Central Directory boundaries in ZIP")
	}

	sec1 := zipBytes[:cdOffset]
	sec2 := zipBytes[cdOffset : cdOffset+cdSize]
	sec3 := make([]byte, len(zipBytes)-eocdOffset)
	copy(sec3, zipBytes[eocdOffset:])
	// EOCD in sec3 must have CD offset set to cdOffset (len(sec1))
	binary.LittleEndian.PutUint32(sec3[16:20], uint32(len(sec1)))

	// 2. Compute 1-MB chunked digests
	chunkHashes := computeSectionChunkHashes([][]byte{sec1, sec2, sec3})

	// Top-level content digest: 0x5a + uint32(num_chunks) + chunkHashes
	var topDigestBuf bytes.Buffer
	topDigestBuf.WriteByte(0x5a)
	_ = binary.Write(&topDigestBuf, binary.LittleEndian, uint32(len(chunkHashes)/32))
	topDigestBuf.Write(chunkHashes)
	contentDigest := sha256.Sum256(topDigestBuf.Bytes())

	// 3. Build signed_data:
	// digests: uint32(len) + (uint32(len) + uint32(algo) + uint32(len) + 32-byte digest)
	// certificates: uint32(len) + (uint32(len) + certDER)
	// additional_attributes: uint32(0)
	var digestsBuf bytes.Buffer
	var digestRecord bytes.Buffer
	_ = binary.Write(&digestRecord, binary.LittleEndian, sigAlgoRSASHA256)
	_ = binary.Write(&digestRecord, binary.LittleEndian, uint32(len(contentDigest)))
	digestRecord.Write(contentDigest[:])

	_ = binary.Write(&digestsBuf, binary.LittleEndian, uint32(digestRecord.Len()))
	digestsBuf.Write(digestRecord.Bytes())

	var certsBuf bytes.Buffer
	_ = binary.Write(&certsBuf, binary.LittleEndian, uint32(len(cert.Raw)))
	certsBuf.Write(cert.Raw)

	var signedDataBuf bytes.Buffer
	_ = binary.Write(&signedDataBuf, binary.LittleEndian, uint32(digestsBuf.Len()))
	signedDataBuf.Write(digestsBuf.Bytes())
	_ = binary.Write(&signedDataBuf, binary.LittleEndian, uint32(certsBuf.Len()))
	signedDataBuf.Write(certsBuf.Bytes())
	_ = binary.Write(&signedDataBuf, binary.LittleEndian, uint32(0)) // additional_attributes length = 0

	signedData := signedDataBuf.Bytes()

	// 4. Sign signed_data with RSA-PKCS1-v1.5 SHA-256
	sdHash := sha256.Sum256(signedData)
	rsaSig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sdHash[:])
	if err != nil {
		return nil, fmt.Errorf("signing APK v2 signed_data: %w", err)
	}

	var sigRecord bytes.Buffer
	_ = binary.Write(&sigRecord, binary.LittleEndian, sigAlgoRSASHA256)
	_ = binary.Write(&sigRecord, binary.LittleEndian, uint32(len(rsaSig)))
	sigRecord.Write(rsaSig)

	var signaturesBuf bytes.Buffer
	_ = binary.Write(&signaturesBuf, binary.LittleEndian, uint32(sigRecord.Len()))
	signaturesBuf.Write(sigRecord.Bytes())

	// Public key in SubjectPublicKeyInfo DER format
	pubKeyDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling public key: %w", err)
	}

	// Signer block
	var signerBuf bytes.Buffer
	_ = binary.Write(&signerBuf, binary.LittleEndian, uint32(len(signedData)))
	signerBuf.Write(signedData)
	_ = binary.Write(&signerBuf, binary.LittleEndian, uint32(signaturesBuf.Len()))
	signerBuf.Write(signaturesBuf.Bytes())
	_ = binary.Write(&signerBuf, binary.LittleEndian, uint32(len(pubKeyDER)))
	signerBuf.Write(pubKeyDER)

	var signersBuf bytes.Buffer
	_ = binary.Write(&signersBuf, binary.LittleEndian, uint32(signerBuf.Len()))
	signersBuf.Write(signerBuf.Bytes())

	var v2SchemeValue bytes.Buffer
	_ = binary.Write(&v2SchemeValue, binary.LittleEndian, uint32(signersBuf.Len()))
	v2SchemeValue.Write(signersBuf.Bytes())

	// 5. Build APK Signing Block:
	// Total size excludes the first uint64 size field!
	// Total size = len(pairs) + 8 (size2) + 16 (magic)
	pairData := v2SchemeValue.Bytes()
	pairLen := uint64(4 + len(pairData)) // uint32 ID + value

	var pairsBuf bytes.Buffer
	_ = binary.Write(&pairsBuf, binary.LittleEndian, pairLen)
	_ = binary.Write(&pairsBuf, binary.LittleEndian, apkSigSchemeV2BlockID)
	pairsBuf.Write(pairData)

	blockSize := uint64(pairsBuf.Len() + 8 + len(apkSigBlockMagic))

	var sigBlock bytes.Buffer
	_ = binary.Write(&sigBlock, binary.LittleEndian, blockSize)
	sigBlock.Write(pairsBuf.Bytes())
	_ = binary.Write(&sigBlock, binary.LittleEndian, blockSize)
	sigBlock.Write(apkSigBlockMagic)

	// 6. Update EOCD Central Directory offset
	newCDOffset := uint32(len(sec1) + sigBlock.Len())
	finalEOCD := make([]byte, len(sec3))
	copy(finalEOCD, sec3)
	binary.LittleEndian.PutUint32(finalEOCD[16:20], newCDOffset)

	// Combine into final APK
	var finalAPK bytes.Buffer
	finalAPK.Write(sec1)
	finalAPK.Write(sigBlock.Bytes())
	finalAPK.Write(sec2)
	finalAPK.Write(finalEOCD)

	return finalAPK.Bytes(), nil
}

func computeSectionChunkHashes(sections [][]byte) []byte {
	const chunkSize = 1048576 // 1 MB
	var hashes bytes.Buffer

	for _, sec := range sections {
		for off := 0; off < len(sec); off += chunkSize {
			end := off + chunkSize
			if end > len(sec) {
				end = len(sec)
			}
			chunk := sec[off:end]

			var chunkBuf bytes.Buffer
			chunkBuf.WriteByte(0xa5)
			_ = binary.Write(&chunkBuf, binary.LittleEndian, uint32(len(chunk)))
			chunkBuf.Write(chunk)

			h := sha256.Sum256(chunkBuf.Bytes())
			hashes.Write(h[:])
		}
	}
	return hashes.Bytes()
}
