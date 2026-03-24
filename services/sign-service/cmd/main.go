package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"wallet-saas-v2/services/sign-service/internal/bootstrap"
	"wallet-saas-v2/services/sign-service/internal/config"
	"wallet-saas-v2/services/sign-service/internal/hsm"
)

func main() {
	if len(os.Args) > 1 && strings.EqualFold(strings.TrimSpace(os.Args[1]), "vault") {
		if err := runVaultCLI(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}

func runVaultCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sign-service vault <import|export|rotate> [flags]")
	}
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	backend, err := bootstrap.OpenBackend(cfg)
	if err != nil {
		return err
	}
	defer backend.Close()
	if err := bootstrap.BootstrapSeeds(backend, cfg); err != nil {
		return err
	}
	manager, ok := backend.(hsm.SeedManager)
	if !ok {
		return fmt.Errorf("backend %q does not support seed import/export/rotate", cfg.HSMBackend)
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "import":
		return runVaultImport(cfg, manager, args[1:])
	case "export":
		return runVaultExport(cfg, manager, args[1:])
	case "rotate":
		return runVaultRotate(cfg, manager, args[1:])
	default:
		return fmt.Errorf("unsupported vault command: %s", args[0])
	}
}

func runVaultImport(cfg config.Config, manager hsm.SeedManager, args []string) error {
	fs := flag.NewFlagSet("vault import", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	tenantID := fs.String("tenant", "", "tenant id")
	signType := fs.String("sign-type", "", "sign type")
	slotID := fs.String("slot-id", "", "explicit slot id")
	seedHex := fs.String("seed-hex", "", "seed hex")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedSlotID, err := resolveSlotID(cfg, *slotID, *tenantID, *signType)
	if err != nil {
		return err
	}
	seed, err := decodeSeedHex(*seedHex)
	if err != nil {
		return err
	}
	if err := manager.ProvisionSeed(resolvedSlotID, seed); err != nil {
		return err
	}
	fmt.Printf("imported slot=%s\n", resolvedSlotID)
	return nil
}

func runVaultExport(cfg config.Config, manager hsm.SeedManager, args []string) error {
	fs := flag.NewFlagSet("vault export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	tenantID := fs.String("tenant", "", "tenant id")
	signType := fs.String("sign-type", "", "sign type")
	slotID := fs.String("slot-id", "", "explicit slot id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedSlotID, err := resolveSlotID(cfg, *slotID, *tenantID, *signType)
	if err != nil {
		return err
	}
	seed, err := manager.ExportSeed(resolvedSlotID)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", hex.EncodeToString(seed))
	return nil
}

func runVaultRotate(cfg config.Config, manager hsm.SeedManager, args []string) error {
	fs := flag.NewFlagSet("vault rotate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	tenantID := fs.String("tenant", "", "tenant id")
	signType := fs.String("sign-type", "", "sign type")
	slotID := fs.String("slot-id", "", "explicit slot id")
	seedHex := fs.String("seed-hex", "", "replacement seed hex")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedSlotID, err := resolveSlotID(cfg, *slotID, *tenantID, *signType)
	if err != nil {
		return err
	}
	seed, err := decodeSeedHex(*seedHex)
	if err != nil {
		return err
	}
	if err := manager.ReplaceSeed(resolvedSlotID, seed); err != nil {
		return err
	}
	fmt.Printf("rotated slot=%s\n", resolvedSlotID)
	return nil
}

func resolveSlotID(cfg config.Config, slotID, tenantID, signType string) (string, error) {
	slotID = strings.TrimSpace(slotID)
	if slotID != "" {
		return slotID, nil
	}
	tenantID = strings.TrimSpace(tenantID)
	signType = strings.TrimSpace(signType)
	if tenantID == "" || signType == "" {
		return "", fmt.Errorf("tenant and sign-type are required unless slot-id is provided")
	}
	return hsm.BuildTenantSlotID(cfg.HSMSlotPrefix, tenantID, signType), nil
}

func decodeSeedHex(seedHex string) ([]byte, error) {
	seedHex = strings.TrimPrefix(strings.TrimSpace(seedHex), "0x")
	if seedHex == "" {
		return nil, fmt.Errorf("seed-hex is required")
	}
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		return nil, err
	}
	if len(seed) == 0 {
		return nil, fmt.Errorf("decoded seed is empty")
	}
	return seed, nil
}
