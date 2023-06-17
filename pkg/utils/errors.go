package utils

import (
	"errors"
	"strings"
)

var (
	ErrNotFound                   = errors.New("resource was not found")
	ErrInsufficientCidrBlocks     = errors.New("InsufficientCidrBlocks: The specified subnet does not have enough free cidr blocks to satisfy the request")
	ErrMsgProviderAndPoolNotFound = "cannot find the instance provider and pool from the cache"
	NotRetryErrors                = []string{InsufficientCidrBlocksReason}
)

// ShouldRetryOnError returns true if the error is retryable, else returns false
func ShouldRetryOnError(err error) bool {
	for _, e := range NotRetryErrors {
		if strings.HasPrefix(err.Error(), e) {
			return false
		}
	}
	return true
}
