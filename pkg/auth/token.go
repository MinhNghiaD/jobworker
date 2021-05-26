package auth

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	token "github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GenerateToken generates a token associated with a claim, signed by ECDSA private signing key.
func GenerateToken(claim *token.Claims, certificate tls.Certificate) (string, error) {
	// Using ECDSA key
	sig := token.Signature{
		Type: jwt.SigningMethodES256,
		Key:  certificate.PrivateKey,
	}

	opts := token.Options{
		Expiration: time.Now().AddDate(0, 1, 0).Unix(),
	}

	return token.Token(claim, &sig, &opts)
}

// DecodeToken uses the certificate associated with the signing key to decrypt the token and returns the original claims
func DecodeToken(raw string, certificate *x509.Certificate) (claims *token.Claims, err error) {
	if certificate == nil {
		return nil, fmt.Errorf("certificate is not provided")
	}

	defer func() {
		// Fail-safe in case of invalid memory access during decoding
		if r := recover(); r != nil {
			err = fmt.Errorf("Decode is panic")
		}
	}()

	// Using ECDSA signing algorithm
	jwtToken, err := jwt.Parse(raw, func(jwtToken *jwt.Token) (interface{}, error) {
		if _, ok := jwtToken.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, status.Errorf(codes.PermissionDenied, "Fail to decrypt token")
		}

		return certificate.PublicKey, nil
	})

	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, err.Error())
	}

	if !jwtToken.Valid {
		return nil, status.Errorf(codes.PermissionDenied, "Token expired")
	}

	claimsMap, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "Fail to get claims")
	}

	return mapToClaims(claimsMap), nil
}

// mapToClaims reconverts claimsMap into Claims object
func mapToClaims(claimsMap jwt.MapClaims) *token.Claims {
	claims := &token.Claims{
		Issuer:  claimsMap["iss"].(string),
		Subject: claimsMap["sub"].(string),
		Name:    claimsMap["name"].(string),
		Email:   claimsMap["email"].(string),
	}

	if roles, ok := claimsMap["roles"].([]interface{}); ok {
		claims.Roles = make([]string, len(roles))
		for i, r := range roles {
			claims.Roles[i] = fmt.Sprint(r)
		}
	}

	if groups, ok := claimsMap["groups"].([]interface{}); ok {
		claims.Groups = make([]string, len(groups))
		for i, g := range groups {
			claims.Groups[i] = fmt.Sprint(g)
		}
	}

	return claims
}
