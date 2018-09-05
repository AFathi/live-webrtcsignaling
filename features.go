package main

import (
	"context"

	"github.com/heytribe/live-webrtcsignaling/my"
)

type Features struct {
	my.NamedRWMutex
	data map[string]string
}

func NewFeatures() *Features {
	features := new(Features)
	features.data = make(map[string]string)
	features.NamedRWMutex.Init("Features")
	return features
}

func (f *Features) GetVariant(ctx context.Context, key string) string {
	f.RLock(ctx)
	s := f.data[key]
	f.RUnlock(ctx)
	return s
}

func (f *Features) IsActive(ctx context.Context, key string) bool {
	s := f.GetVariant(ctx, key)
	if s == "0" || s == "" || s == "false" {
		return false
	}
	return true
}

func (f *Features) Register(ctx context.Context, key string, value string) {
	f.RLock(ctx)
	f.data[key] = value
	f.RUnlock(ctx)
}
