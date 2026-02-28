package service

import "context"

func (s *ChainService) ConvertAddress(ctx context.Context, chain, network, addrType, publicKey string) (string, error) {
	if err := validateChainNetwork(chain, network); err != nil {
		return "", err
	}
	binding, err := s.Router.Resolve(chain, network)
	if err != nil {
		return "", err
	}
	return withRetry(ctx, "convert-address", func() (string, error) {
		return binding.Adapter.ConvertAddress(ctx, binding.Chain, network, addrType, publicKey)
	})
}
