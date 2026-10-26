// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"testing"
)

func TestSignAPKVerification(t *testing.T) {
	entries := map[string][]byte{
		"AndroidManifest.xml": []byte("test manifest"),
		"classes.dex":         []byte("test dex"),
	}

	mf, sf, rsaBytes, err := SignAPK(entries)
	if err != nil {
		t.Fatalf("SignAPK failed: %v", err)
	}

	if len(mf) == 0 || len(sf) == 0 || len(rsaBytes) == 0 {
		t.Fatalf("empty signature files")
	}

	t.Logf("MANIFEST.MF:\n%s", string(mf))
	t.Logf("CERT.SF:\n%s", string(sf))
	t.Logf("CERT.RSA length: %d", len(rsaBytes))

	// Verify that CERT.RSA can be parsed as PKCS#7
	var ci pkcs7ContentInfo
	rest, err := asn1.Unmarshal(rsaBytes, &ci)
	if err != nil {
		t.Fatalf("failed to unmarshal ContentInfo: %v", err)
	}
	if len(rest) > 0 {
		t.Fatalf("unexpected trailing bytes in ContentInfo: %d bytes", len(rest))
	}

	var sd pkcs7SignedData
	rest, err = asn1.Unmarshal(ci.Content.Bytes, &sd)
	if err != nil {
		t.Fatalf("failed to unmarshal SignedData: %v", err)
	}
	if len(rest) > 0 {
		t.Fatalf("unexpected trailing bytes in SignedData: %d bytes", len(rest))
	}

	if len(sd.SignerInfos) == 0 {
		t.Fatalf("no SignerInfos in SignedData")
	}

	si := sd.SignerInfos[0]
	// Extract certificate
	cert, err := x509.ParseCertificate(sd.Certificates.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	// Verify the signature on CERT.SF!
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("public key is not RSA")
	}

	sfHash := sha256.Sum256(sf)
	err = rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, sfHash[:], si.EncryptedDigest)
	if err != nil {
		t.Fatalf("RSA signature verification on CERT.SF failed: %v", err)
	}
	t.Logf("RSA signature successfully verified against certificate!")
}
