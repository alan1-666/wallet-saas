package solana

import "testing"

func TestTokenAccountsByOwnerResultTotalAmount(t *testing.T) {
	result := solanaTokenAccountsByOwnerResult{
		Value: []solanaTokenAccountEntry{
			{},
			{
				Account: struct {
					Data struct {
						Parsed struct {
							Info struct {
								TokenAmount struct {
									Amount string `json:"amount"`
								} `json:"tokenAmount"`
							} `json:"info"`
						} `json:"parsed"`
					} `json:"data"`
				}{},
			},
		},
	}
	result.Value[0].Account.Data.Parsed.Info.TokenAmount.Amount = "100"
	result.Value[1].Account.Data.Parsed.Info.TokenAmount.Amount = "25"

	got, err := result.totalAmount()
	if err != nil {
		t.Fatalf("totalAmount returned error: %v", err)
	}
	if got != "125" {
		t.Fatalf("unexpected total amount: %s", got)
	}
}

func TestTokenAccountsByOwnerResultTotalAmountRejectsInvalidAmount(t *testing.T) {
	result := solanaTokenAccountsByOwnerResult{
		Value: []solanaTokenAccountEntry{{}},
	}
	result.Value[0].Account.Data.Parsed.Info.TokenAmount.Amount = "bad"
	if _, err := result.totalAmount(); err == nil {
		t.Fatalf("expected invalid token amount error")
	}
}
