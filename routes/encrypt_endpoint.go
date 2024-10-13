package routes

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v4"
	"jwe-go/model"
	"jwe-go/packages/crypto"
	"jwe-go/packages/json"
	"jwe-go/packages/schema"
	"net/http"
)

func EncryptEndpoint(context *gin.Context) {
	var encryption model.EncryptRequest

	// Use Jsoniter for decoding the JSON body
	body, err := context.GetRawData()
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Use strict unmarshaling
	if err := json.StrictUnmarshal(body, &encryption); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON or unknown field"})
		return
	}

	// Manually validate the struct using the validator
	if err := schema.Validate.Struct(encryption); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Import the public key from PEM
	publicKey, err := crypto.ImportRSAPublicKeyFromPEM(encryption.PublicKeyPem)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert the RSA public key to a JWK
	jwk, err := crypto.ConvertRSAPublicKeyToJWK(publicKey)
	if err != nil {
		context.JSON(
			http.StatusInternalServerError,
			gin.H{"error": fmt.Errorf("error converting public key to JWK:: %v", err).Error()},
		)
		return
	}

	// Compute thumbprint of the public key (SHA256)
	thumbprint, err := crypto.GetJWKThumbprint(jwk)
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compute public key thumbprint"})
		return
	}

	// Create JWE Encrypter with RSA-OAEP-256 and AES-GCM
	encrypter, err := jose.NewEncrypter(
		jose.A256GCM, // Content encryption algorithm
		jose.Recipient{
			Algorithm: jose.RSA_OAEP_256, // Key encryption algorithm
			Key:       publicKey,         // Recipient's public key
		},
		(&jose.EncrypterOptions{}).WithHeader("server_kid", thumbprint), // Add custom header (server_kid)
	)
	if err != nil {
		context.JSON(
			http.StatusInternalServerError,
			gin.H{"error": fmt.Errorf("error creating encrypter: %v", err).Error()},
		)
		return
	}

	// Encrypt the data
	jwe, err := encrypter.Encrypt([]byte(encryption.Plaintext))
	if err != nil {
		context.JSON(
			http.StatusInternalServerError,
			gin.H{"error": fmt.Errorf("error encrypting data: %v", err).Error()},
		)
		return
	}

	// Serialize JWE to a compact format
	serialized, err := jwe.CompactSerialize()
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	context.String(http.StatusOK, serialized)
}
