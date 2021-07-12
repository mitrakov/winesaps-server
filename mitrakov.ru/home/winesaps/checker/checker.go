package checker

import "crypto"
import "crypto/rsa"
import "crypto/x509"
import "encoding/pem"
import "encoding/base64"
import . "mitrakov.ru/home/winesaps/utils" // nolint

// SignatureChecker is a special crypto component to verify signature of data.
// It uses x.509 specification for RSA protocol. A valid RSA public key should be provided.
// This component is independent.
type SignatureChecker struct {
    key *rsa.PublicKey
}

// NewSignatureChecker creates a new SignatureChecker. Please do not create a SignatureChecker directly.
// "publicKey" - RSA public key
func NewSignatureChecker(publicKey string) (*SignatureChecker, *Error) {
    block, _ := pem.Decode([]byte(publicKey))
    if block != nil {
        pub, err := x509.ParsePKIXPublicKey(block.Bytes)
        if err == nil {
            if key, ok := pub.(*rsa.PublicKey); ok {
                return &SignatureChecker{key}, nil
            }
            return nil, NewErr(&SignatureChecker{}, 25, "Key is not RSA Public Key")
        }
        return nil, NewErrFromError(&SignatureChecker{}, 26, err)
    }
    return nil, NewErr(&SignatureChecker{}, 27, "Cannot decode public key")
}

// CheckSignature verifies the signature of a "data" string with "signature" by RSASSA-PKCS1-v1_5 scheme.
// "data" - string to verify
// "signature" - string containing the signature of data that was signed with the private key of the developer.
func (checker SignatureChecker) CheckSignature(data, signature string) *Error {
    // calculate SHA1
    hash := crypto.SHA1.New()
    _, err := hash.Write([]byte(data))
    hashed := hash.Sum(nil)
    
    // decode signature & verify
    if err == nil {
        var sig []byte
        sig, err = base64.StdEncoding.DecodeString(signature)
        if err == nil {
            err = rsa.VerifyPKCS1v15(checker.key, crypto.SHA1, hashed, sig)
        }
    }
    return NewErrFromError(checker, 28, err)
}
