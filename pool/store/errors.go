package store

import "errors"

// ErrInvalidNonce is returned when a signed request contains an invalid nonce.d
var ErrInvalidNonce = errors.New("invalid nonce")
