// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

// NewStorageConfigFromLoader creates a StorageConfig from a config loader.
func NewStorageConfigFromLoader(loader ConfigLoader) *StorageConfig {
	return &StorageConfig{
		Backend:           loader.TTSStorageBackend(),
		BasePath:          loader.TTSStoragePath(),
		R2Endpoint:        loader.TTSR2Endpoint(),
		R2AccessKeyID:     loader.TTSR2AccessKeyID(),
		R2SecretAccessKey: loader.TTSR2SecretAccessKey(),
		R2Bucket:          loader.TTSR2Bucket(),
		R2PublicURL:       loader.TTSR2PublicURL(),
	}
}
