package auth

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	token "github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
)

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

func DecodeToken(raw string, certificate *x509.Certificate) (*token.Claims, error) {
	// Using ECDSA key
	jwtToken, err := jwt.Parse(raw, func(jwtToken *jwt.Token) (interface{}, error) {
		if _, ok := jwtToken.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected method: %s", jwtToken.Header["alg"])
		}

		return certificate.PublicKey, nil
	})

	if err != nil {
		return nil, err
	}

	claimsMap, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("Cannot get claims")
	}

	return mapToClaims(claimsMap), nil
}

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
