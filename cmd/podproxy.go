package cmd

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/angelini/fusion/pkg/podproxy"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdPodProxy() *cobra.Command {
	var (
		port          int
		publicKeyPath string
	)

	cmd := &cobra.Command{
		Use:   "podproxy",
		Short: "Proxies requests to the correct pod",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := ctx.Value(logKey).(*zap.Logger)

			log.Info("start pod proxy", zap.Int("port", port))

			publicKey, err := parsePublicKey(publicKeyPath)
			if err != nil {
				return err
			}

			proxy, err := podproxy.NewProxy(log, "fusion", "fusion-manager-service.fusion.svc.cluster.local", port, publicKey)
			if err != nil {
				return err
			}

			return proxy.Start(ctx)
		},
	}

	cmd.PersistentFlags().IntVarP(&port, "port", "p", 5152, "Pod proxy port")
	cmd.PersistentFlags().StringVar(&publicKeyPath, "public", "secrets/paseto.pub", "Paseto public key")

	return cmd
}

func parsePublicKey(path string) (ed25519.PublicKey, error) {
	pubKeyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open Paseto public key file: %w", err)
	}

	block, _ := pem.Decode(pubKeyBytes)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("error decoding Paseto public key PEM")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing Paseto public key: %w", err)
	}

	return pub.(ed25519.PublicKey), nil
}
