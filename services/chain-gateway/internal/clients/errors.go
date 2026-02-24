package clients

import "fmt"

func NewRPCError(op, msg string) error {
	if msg == "" {
		return fmt.Errorf("%s failed", op)
	}
	return fmt.Errorf("%s failed: %s", op, msg)
}
