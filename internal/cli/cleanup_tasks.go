// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cli // import "miniflux.app/v2/internal/cli"

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"miniflux.app/v2/internal/config"
	"miniflux.app/v2/internal/metric"
	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/storage"
)

func runCleanupTasks(store *storage.Storage) {
	nbSessions := store.CleanOldSessions(config.Opts.CleanupRemoveSessionsInterval())
	nbUserSessions := store.CleanOldUserSessions(config.Opts.CleanupRemoveSessionsInterval())
	slog.Info("Sessions cleanup completed",
		slog.Int64("application_sessions_removed", nbSessions),
		slog.Int64("user_sessions_removed", nbUserSessions),
	)

	// TTS cache cleanup
	if config.Opts.TTSEnabled() {
		filePaths, err := store.CleanupExpiredTTSCache()
		if err != nil {
			slog.Error("Unable to cleanup expired TTS cache", slog.Any("error", err))
		} else if len(filePaths) > 0 {
			// Delete physical files
			storagePath := config.Opts.TTSStoragePath()
			deletedCount := 0
			for _, relPath := range filePaths {
				fullPath := filepath.Join(storagePath, relPath)
				if err := os.Remove(fullPath); err != nil {
					slog.Warn("Unable to delete TTS audio file",
						slog.String("file_path", fullPath),
						slog.Any("error", err),
					)
				} else {
					deletedCount++
				}
			}
			slog.Info("TTS cache cleanup completed",
				slog.Int("database_entries_removed", len(filePaths)),
				slog.Int("files_deleted", deletedCount),
			)
		} else {
			slog.Debug("No expired TTS cache entries to cleanup")
		}
	}

	startTime := time.Now()
	if rowsAffected, err := store.ArchiveEntries(model.EntryStatusRead, config.Opts.CleanupArchiveReadInterval(), config.Opts.CleanupArchiveBatchSize()); err != nil {
		slog.Error("Unable to archive read entries", slog.Any("error", err))
	} else {
		slog.Info("Archiving read entries completed",
			slog.Int64("read_entries_archived", rowsAffected),
		)

		if config.Opts.HasMetricsCollector() {
			metric.ArchiveEntriesDuration.WithLabelValues(model.EntryStatusRead).Observe(time.Since(startTime).Seconds())
		}
	}

	startTime = time.Now()
	if rowsAffected, err := store.ArchiveEntries(model.EntryStatusUnread, config.Opts.CleanupArchiveUnreadInterval(), config.Opts.CleanupArchiveBatchSize()); err != nil {
		slog.Error("Unable to archive unread entries", slog.Any("error", err))
	} else {
		slog.Info("Archiving unread entries completed",
			slog.Int64("unread_entries_archived", rowsAffected),
		)

		if config.Opts.HasMetricsCollector() {
			metric.ArchiveEntriesDuration.WithLabelValues(model.EntryStatusUnread).Observe(time.Since(startTime).Seconds())
		}
	}

	if enclosuresAffected, err := store.DeleteEnclosuresOfRemovedEntries(); err != nil {
		slog.Error("Unable to delete enclosures from removed entries", slog.Any("error", err))
	} else {
		slog.Info("Deleting enclosures from removed entries completed",
			slog.Int64("removed_entries_enclosures_deleted", enclosuresAffected))
	}

	if contentAffected, err := store.ClearRemovedEntriesContent(config.Opts.CleanupArchiveBatchSize()); err != nil {
		slog.Error("Unable to clear content from removed entries", slog.Any("error", err))
	} else {
		slog.Info("Clearing content from removed entries completed",
			slog.Int64("removed_entries_content_cleared", contentAffected))
	}
}
