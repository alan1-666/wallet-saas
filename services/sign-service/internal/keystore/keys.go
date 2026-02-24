package keystore

type Keys struct {
	store *levelStore
}

func New(path string) (*Keys, error) {
	store, err := newLevelStore(path)
	if err != nil {
		return nil, err
	}
	return &Keys{store: store}, nil
}

func (k *Keys) Close() error {
	if k == nil || k.store == nil {
		return nil
	}
	return k.store.Close()
}

func (k *Keys) GetPrivKey(publicKey string) (string, bool) {
	key := []byte(publicKey)
	data, err := k.store.Get(key)
	if err != nil {
		return "", false
	}
	return toString(data), true
}

func (k *Keys) StoreKeys(keyList []Key) bool {
	for _, item := range keyList {
		key := []byte(item.CompressPubkey)
		value := toBytes(item.PrivateKey)
		if err := k.store.Put(key, value); err != nil {
			return false
		}
	}
	return true
}
