package rsa

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/gob"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/common/types"
)

func readPrivateKey(reader KeysReader, fileName string) (*rsa.PrivateKey, error) {
	block, err := readPEMBlock(reader, fileName)
	if err != nil {
		return nil, fmt.Errorf("fail to read PEM block from file %s with error: %w", fileName, err)
	}

	if block.Type != types.PrivateKeyType {
		return nil, fmt.Errorf("non-private key PEM block type '%s' from file %s", block.Type, fileName)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key with error: %w", err)
	}

	return privateKey, err
}

func readPublicKey(reader KeysReader, fileName string) (*rsa.PublicKey, error) {
	block, err := readPEMBlock(reader, fileName)
	if err != nil {
		return nil, fmt.Errorf("fail to read PEM block from file %s with error: %w", fileName, err)
	}

	if block.Type != types.PublicKeyType {
		return nil, fmt.Errorf("non-public key PEM block type '%s' from file %s", block.Type, fileName)
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing public key with error: %w", err)
	}

	return publicKey.(*rsa.PublicKey), nil
}

func readPEMBlock(reader KeysReader, fileName string) (*pem.Block, error) {
	contents, err := reader.ReadKey(fileName)
	if err != nil {
		return nil, fmt.Errorf("read file %s with error: %w", fileName, err)
	}

	block, _ := pem.Decode(contents)
	if block == nil {
		return nil, fmt.Errorf("no PEM encoded key found in %s", fileName)
	}
	return block, nil
}

func GenerateData(reader KeysReader, privateKeyfile string, payload types.RSAPayload) (string, error) {
	var err error
	var privateKey *rsa.PrivateKey

	privateKey, err = readPrivateKey(reader, privateKeyfile)
	if err != nil {
		return "", fmt.Errorf("fail to read private key file %s with error: %w", privateKeyfile, err)
	}

	gob.Register(types.RSAPayload{})

	payloadBuffer := bytes.Buffer{}
	payloadEncoder := gob.NewEncoder(&payloadBuffer)
	if err = payloadEncoder.Encode(&payload); err != nil {
		return "", fmt.Errorf("fail to encode payload, error: %w", err)
	}

	hashed := sha256.Sum256(payloadBuffer.Bytes())
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("signing payload with error: %w", err)
	}

	l := types.RSAInfo{
		Payload:   payloadBuffer.Bytes(),
		Signature: signature,
	}

	dataBuffer := bytes.Buffer{}
	dataEncoder := gob.NewEncoder(&dataBuffer)
	if err = dataEncoder.Encode(l); err != nil {
		return "", fmt.Errorf("fail to encode data, error: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(dataBuffer.Bytes())

	result := types.PEMHeader + "\n"

	width := 64
	for i := 0; ; i += width {
		if i+width <= len(b64) {
			result += b64[i:i+width] + "\n"
		} else {
			result += b64[i:] + "\n"
			break
		}
	}
	result += types.PEMFooter

	return result, nil
}

func VerifyData(reader KeysReader, publicKeyFile, data string) (*types.RSAPayload, error) {
	var err error
	var publicKey *rsa.PublicKey

	publicKey, err = readPublicKey(reader, publicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("fail to read public key file  %s with error:  %w", publicKeyFile, err)
	}

	dataStr := strings.TrimSpace(data)

	if !strings.HasPrefix(dataStr, types.PEMHeader) || !strings.HasSuffix(dataStr, types.PEMFooter) {
		return nil, errors.New("invalid data in PEM format")
	}

	base64Str := strings.Replace(dataStr[len(types.PEMHeader):len(dataStr)-len(types.PEMFooter)], "\n", "", -1)

	gob.Register(types.RSAPayload{})

	var rsaInfo types.RSAInfo
	dataBytes, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, fmt.Errorf("fail to decode data base64 string with error: %w", err)
	}
	dataBuffer := bytes.Buffer{}
	dataBuffer.Write(dataBytes)
	dataEncoder := gob.NewDecoder(&dataBuffer)
	if err = dataEncoder.Decode(&rsaInfo); err != nil {
		return nil, err
	}

	hashed := sha256.Sum256(rsaInfo.Payload)

	if err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], rsaInfo.Signature); err != nil {
		return nil, err
	}

	var payload types.RSAPayload
	payloadBuffer := bytes.Buffer{}
	payloadBuffer.Write(rsaInfo.Payload)
	payloadDecoder := gob.NewDecoder(&payloadBuffer)
	if err = payloadDecoder.Decode(&payload); err != nil {
		return nil, err
	}

	return &payload, nil
}
