package meme

import "time"

const maxPresignTTL = 15 * time.Minute

const (
	defaultLimit  = 20
	maxLimit      = 100
	maxUploadSize = 10 * 1024 * 1024 // 10 MB
)
