package keystore

import (
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
)

type Keys struct {
	store     *levelStore
	namespace string
	mu        sync.Mutex
}

const masterSeedKeyPrefix = "__hsm_slot__:"

func New(path, namespace string) (*Keys, error) {
	store, err := newLevelStore(path)
	if err != nil {
		return nil, err
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = "software"
	}
	return &Keys{store: store, namespace: namespace}, nil
}

func (k *Keys) Close() error {
	if k == nil || k.store == nil {
		return nil
	}
	return k.store.Close()
}

func (k *Keys) LoadOrCreateSeed(slotID string) ([]byte, error) {
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
	if seed, err := k.store.Get(slotKey); err == nil && len(seed) > 0 {
		return append([]byte(nil), seed...), nil
	}

	seed := make([]byte, 64)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	if err := k.store.Put(slotKey, seed); err != nil {
		return nil, err
	}
	return append([]byte(nil), seed...), nil
}

func (k *Keys) storageKey(slotID string) string {
	return strings.TrimSpace(k.namespace) + ":" + strings.TrimSpace(slotID)
}
