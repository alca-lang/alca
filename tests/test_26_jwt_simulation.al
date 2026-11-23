// Copyright 2026 Pawl Programming Language
// Test 26: JWT (JSON Web Token) simulation - signing, validation, claims.

use std.json
use std.crypto

// 1. JWT structure simulation
struct JWT {
    header: string
    payload: string
    signature: string

// 2. Base64URL encoding (for JWT)
}
func base64url_encode(data: string) -> string {
    // Use standard base64, then replace + with -, / with _, remove =
    b64 := crypto.base64_encode(data)
    b64 = string.replace(b64, "+", "-")
    b64 = string.replace(b64, "/", "_")
    b64 = string.replace(b64, "=", "")
    return b64
}
func base64url_decode(data: string) -> string {
    // Reverse the encoding
    decoded := string.replace(data, "-", "+")
    decoded = string.replace(decoded, "_", "/")
    // Add padding if needed
    while len(decoded) % 4 != 0 {
        decoded = decoded + "="
    }
    return crypto.base64_decode(decoded)

// 3. Create JWT header
}
func create_header(algorithm: string, typ: string) -> string {
    header_map := {"alg": algorithm, "typ": typ}
    return json.encode(header_map)
}
header := create_header("HS256", "JWT")
assert string.contains(header, "HS256")
assert string.contains(header, "JWT")

// 4. Create JWT payload (claims)
func create_payload(iss: string, sub: string, exp: int, iat: int) -> string {
    payload_map := {
        "iss": iss,
        "sub": sub,
        "exp": string(exp),
        "iat": string(iat)
    }
    return json.encode(payload_map)
}
now := int(time.now())
exp_time := now + 3600 // 1 hour from now
payload := create_payload("pawl-auth", "user123", exp_time, now)
assert string.contains(payload, "user123")

// 5. Sign JWT
func sign_jwt(header: string, payload: string, secret: string) -> string {
    signing_input := base64url_encode(header) + "." + base64url_encode(payload)
    signature := crypto.hmac_sha256(secret, signing_input)
    return base64url_encode(signature)
}
secret_key := "super_secret_jwt_key"
signature := sign_jwt(header, payload, secret_key)
assert len(signature) > 0

// 6. Construct full JWT
func construct_jwt(header: string, payload: string, signature: string) -> string {
    return base64url_encode(header) + "." + base64url_encode(payload) + "." + signature
}
jwt_token := construct_jwt(header, payload, signature)
parts := string.split(jwt_token, ".")
assert len(parts) == 3

// 7. Decode JWT
func decode_jwt(token: string) -> map[string, string] {
    parts := string.split(token, ".")
    if len(parts) != 3 {
        return {}
    }
    header_decoded := base64url_decode(parts[0])
    payload_decoded := base64url_decode(parts[1])
    return {
        "header": header_decoded,
        "payload": payload_decoded,
        "signature": parts[2]
    }
}
decoded := decode_jwt(jwt_token)
assert json.validate(decoded["header"])
assert json.validate(decoded["payload"])

// 8. Verify JWT signature
func verify_jwt(token: string, secret: string) -> bool {
    parts := string.split(token, ".")
    if len(parts) != 3 {
        return false
    }
    header_encoded := parts[0]
    payload_encoded := parts[1]
    signature := parts[2]

    signing_input := header_encoded + "." + payload_encoded
    expected_signature := base64url_encode(crypto.hmac_sha256(secret, signing_input))
    return signature == expected_signature
}
assert verify_jwt(jwt_token, secret_key)
assert !verify_jwt(jwt_token, "wrong_secret")

// 9. Check token expiration
func is_token_expired(payload: string) -> bool {
    payload_map := json.parse(payload)
    exp := int(payload_map["exp"])
    current := int(time.now())
    return current > exp
}
future_payload := create_payload("pawl-auth", "user123", now + 3600, now)
assert !is_token_expired(future_payload)

past_payload := create_payload("pawl-auth", "user123", now - 3600, now)
assert is_token_expired(past_payload)

// 10. Extract claims from JWT
func get_claim(payload: string, claim: string) -> string {
    payload_map := json.parse(payload)
    if claim in payload_map {
        return string(payload_map[claim])
    }
    return ""
}
iss_claim := get_claim(payload, "iss")
assert iss_claim == "pawl-auth"

sub_claim := get_claim(payload, "sub")
assert sub_claim == "user123"

// 11. JWT with custom claims
func create_custom_payload(claims: map[string, string]) -> string {
    return json.encode(claims)
}
custom_claims := {
    "user_id": "123",
    "role": "admin",
    "permissions": "read,write"
}
custom_payload := create_custom_payload(custom_claims)
custom_jwt := construct_jwt(header, custom_payload, sign_jwt(header, custom_payload, secret_key))
assert verify_jwt(custom_jwt, secret_key)

// 12. Refresh token simulation
func refresh_token(old_token: string, secret: string) -> string {
    decoded := decode_jwt(old_token)
    old_payload := decoded["payload"]
    payload_map := json.parse(old_payload)

    // Update expiration
    new_exp := int(time.now()) + 3600
    payload_map["exp"] = string(new_exp)

    new_payload := json.encode(payload_map)
    new_signature := sign_jwt(header, new_payload, secret)
    return construct_jwt(header, new_payload, new_signature)
}
refreshed := refresh_token(jwt_token, secret_key)
assert verify_jwt(refreshed, secret_key)

// 13. Token with audience claim
func create_payload_with_aud(iss: string, sub: string, aud: string, exp: int) -> string {
    payload_map := {
        "iss": iss,
        "sub": sub,
        "aud": aud,
        "exp": string(exp)
    }
    return json.encode(payload_map)
}
aud_payload := create_payload_with_aud("pawl-auth", "user123", "api.pawl.dev", now + 3600)
aud_claim := get_claim(aud_payload, "aud")
assert aud_claim == "api.pawl.dev"

// 14. Validate audience
func validate_audience(payload: string, expected_aud: string) -> bool {
    aud := get_claim(payload, "aud")
    return aud == expected_aud
}
assert validate_audience(aud_payload, "api.pawl.dev")
assert !validate_audience(aud_payload, "wrong.audience")

// 15. JWT with issuer validation
func validate_issuer(payload: string, expected_iss: string) -> bool {
    iss := get_claim(payload, "iss")
    return iss == expected_iss
}
assert validate_issuer(payload, "pawl-auth")
assert !validate_issuer(payload, "wrong-issuer")

// 16. Token ID (jti) claim
func create_payload_with_jti(iss: string, sub: string, jti: string, exp: int) -> string {
    payload_map := {
        "iss": iss,
        "sub": sub,
        "jti": jti,
        "exp": string(exp)
    }
    return json.encode(payload_map)
}
jti := crypto.uuid()
jti_payload := create_payload_with_jti("pawl-auth", "user123", jti, now + 3600)
jti_claim := get_claim(jti_payload, "jti")
assert len(jti_claim) == 36

// 17. Not before (nbf) claim
func create_payload_with_nbf(iss: string, sub: string, nbf: int, exp: int) -> string {
    payload_map := {
        "iss": iss,
        "sub": sub,
        "nbf": string(nbf),
        "exp": string(exp)
    }
    return json.encode(payload_map)
}
nbf_payload := create_payload_with_nbf("pawl-auth", "user123", now, now + 3600)
nbf_claim := get_claim(nbf_payload, "nbf")
assert int(nbf_claim) == now

// 18. Validate not before
func is_token_valid_nbf(payload: string) -> bool {
    nbf := int(get_claim(payload, "nbf"))
    current := int(time.now())
    return current >= nbf
}
assert is_token_valid_nbf(nbf_payload)

// 19. Multiple algorithm support (simulation)
func sign_with_algorithm(header: string, payload: string, secret: string, algorithm: string) -> string {
    signing_input := base64url_encode(header) + "." + base64url_encode(payload)
    if algorithm == "HS256" {
        signature := crypto.hmac_sha256(secret, signing_input)
        return base64url_encode(signature)
    }
    if algorithm == "HS512" {
        signature := crypto.hmac_sha512(secret, signing_input)
        return base64url_encode(signature)
    }
    return ""
}
hs256_header := create_header("HS256", "JWT")
hs256_sig := sign_with_algorithm(hs256_header, payload, secret_key, "HS256")
assert len(hs256_sig) > 0

// 20. Full JWT validation workflow
func validate_jwt_workflow(token: string, secret: string, expected_iss: string, expected_aud: string) -> bool {
    // Verify signature
    if !verify_jwt(token, secret) {
        return false
    // Decode payload
    }
    decoded := decode_jwt(token)
    payload_str := decoded["payload"]
    // Check expiration
    if is_token_expired(payload_str) {
        return false
    // Validate issuer
    }
    if !validate_issuer(payload_str, expected_iss) {
        return false
    // Validate audience (if present)
    }
    aud := get_claim(payload_str, "aud")
    if aud != "" and aud != expected_aud {
        return false
    }
    return true
}
assert validate_jwt_workflow(jwt_token, secret_key, "pawl-auth", "")

println("✓ test_26_jwt_simulation passed")
