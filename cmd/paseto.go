package cmd

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/o1egl/paseto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdPaseto() *cobra.Command {
	var (
		privateKeyPath string
		publicKeyPath  string
	)

	cmd := &cobra.Command{
		Use:   "paseto",
		Short: "Sign a paseto payload",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			payload := args[0]
			log.Info("sign payload", zap.String("payload", payload))

			privateKey, err := readKeyFile(privateKeyPath)
			if err != nil {
				return err
			}

			jsonToken := paseto.JSONToken{
				Audience: "dateilager.fusion",
				Issuer:   "dev",
				Jti:      "dateilager.fusion.dev",
				Subject:  payload,
			}

			v2 := paseto.NewV2()
			token, err := v2.Sign(privateKey, jsonToken, nil)
			if err != nil {
				return err
			}

			fmt.Printf("%s", token)
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&privateKeyPath, "private", "development/paseto.pem", "Paseto private key")
	cmd.PersistentFlags().StringVar(&publicKeyPath, "public", "development/paseto.pub", "Paseto public key")

	return cmd
}

func readKeyFile(path string) (any, error) {
	privateKeyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(privateKeyBytes)
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}
