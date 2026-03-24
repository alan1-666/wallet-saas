package keystore

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
)

type Keys struct {
	store      *levelStore
	namespace  string
	password   string
	autoCreate bool
	mu         sync.Mutex
}

const masterSeedKeyPrefix = "__hsm_slot__:"

var (
	ErrSeedNotProvisioned = errors.New("seed slot is not provisioned")
	ErrSeedAlreadyExists  = errors.New("seed slot already provisioned")
)

func New(path, namespace, password string, autoCreate bool) (*Keys, error) {
	store, err := newLevelStore(path)
	if err != nil {
		return nil, err
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = "software"
	}
	password = strings.TrimSpace(password)
	if password == "" {
		_ = store.Close()
		return nil, fmt.Errorf("vault unlock password is required")
	}
	return &Keys{store: store, namespace: namespace, password: password, autoCreate: autoCreate}, nil
}

func (k *Keys) Close() error {
	if k == nil || k.store == nil {
		return nil
	}
	return k.store.Close()
}

func (k *Keys) LoadOrCreateSeed(slotID string) ([]byte, error) {
	if seed, err := k.LoadSeed(slotID); err == nil {
		return seed, nil
	} else if !errors.Is(err, ErrSeedNotProvisioned) {
		return nil, err
	}
	if !k.autoCreate {
		return nil, ErrSeedNotProvisioned
	}
	seed := make([]byte, 64)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	if err := k.ProvisionSeed(slotID, seed); err != nil {
		return nil, err
	}
	return append([]byte(nil), seed...), nil
}

func (k *Keys) LoadSeed(slotID string) ([]byte, error) {
	if k == nil || k.store == nil {
		return nil, fmt.Errorf("keystore is not initialized")
	}
	k.mu.Lock()
	defer k.mu.Unlock()

	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return nil, fmt.Errorf("slot id is required")
	}
	slotKey := []byte(masterSeedKeyPrefix + k.storageKey(slotID))
	ciphertext, err := k.store.Get(slotKey)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, ErrSeedNotProvisioned
		}
		return nil, err
	}
	if len(ciphertext) == 0 {
		return nil, ErrSeedNotProvisioned
	}
	seed, err := decryptSeed(k.password, k.storageKey(slotID), ciphertext)
	if err != nil {
		return nil, err
	}
	return seed, nil
}

func (k *Keys) ProvisionSeed(slotID string, seed []byte) error {
	if k == nil || k.store == nil {
		return fmt.Errorf("keystore is not initialized")
	}
	k.mu.Lock()
	defer k.mu.Unlock()

	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return fmt.Errorf("slot id is required")
	}
	if len(seed) == 0 {
		return fmt.Errorf("seed is required")
	}
	slotKey := []byte(masterSeedKeyPrefix + k.storageKey(slotID))
	if existing, err := k.store.Get(slotKey); err == nil && len(existing) > 0 {
		return ErrSeedAlreadyExists
	} else if err != nil && !errors.Is(err, leveldb.ErrNotFound) {
		return err
	}
	ciphertext, err := encryptSeed(k.password, k.storageKey(slotID), seed)
	if err != nil {
		return err
	}
	return k.store.Put(slotKey, ciphertext)
}

func (k *Keys) ReplaceSeed(slotID string, seed []byte) error {
	if k == nil || k.store == nil {
		return fmt.Errorf("keystore is not initialized")
	}
	k.mu.Lock()
	defer k.mu.Unlock()

	slotID = strings.TrimSpace(slotID)
	if slotID == "" {
		return fmt.Errorf("slot id is required")
	}
	if len(seed) == 0 {
		return fmt.Errorf("seed is required")
	}
	slotKey := []byte(masterSeedKeyPrefix + k.storageKey(slotID))
	ciphertext, err := encryptSeed(k.password, k.storageKey(slotID), seed)
	if err != nil {
		return err
	}
	return k.store.Put(slotKey, ciphertext)
}

func (k *Keys) storageKey(slotID string) string {
	return strings.TrimSpace(k.namespace) + ":" + strings.TrimSpace(slotID)
}
