package app

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) objectKey(id string, expires bool) string {
	prefix := a.Cfg.MinIO.PrefixTTS
	if expires {
		prefix = a.Cfg.MinIO.PrefixUserAudio
	}
	return strings.Trim(prefix, "/") + "/" + id + ".wav"
}

// The database is the durable upload outbox: no remote object is written before
// the audio metadata/learning transaction commits. Failures keep the local copy.
func (a *App) mediaWorker(ctx context.Context) {
	if a.Objects == nil {
		return
	}
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		a.syncMedia(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
func (a *App) syncMedia(ctx context.Context) {
	for i := 0; i < 10; i++ {
		tx, e := a.DB.Begin(ctx)
		if e != nil {
			return
		}
		var id, path, mime, key string
		var expires bool
		e = tx.QueryRow(ctx, `SELECT id::text,path,mime,object_key,expires_at IS NOT NULL FROM audio_assets WHERE uploaded_at IS NULL AND upload_retry_at<=now() AND (expires_at IS NULL OR expires_at>now()) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &path, &mime, &key, &expires)
		if e != nil {
			tx.Rollback(ctx)
			return
		}
		if key == "" {
			key = a.objectKey(id, expires)
		}
		f, e := os.Open(path)
		if e == nil {
			var info os.FileInfo
			info, e = f.Stat()
			if e == nil {
				call, cancel := context.WithTimeout(ctx, 15*time.Second)
				e = a.Objects.Put(call, key, f, info.Size(), mime)
				cancel()
			}
			f.Close()
		}
		if e == nil {
			_, e = tx.Exec(ctx, "UPDATE audio_assets SET object_key=$1,uploaded_at=now() WHERE id=$2", key, id)
		} else {
			_, e = tx.Exec(ctx, "UPDATE audio_assets SET object_key=$1,upload_retry_at=now()+interval '1 minute' WHERE id=$2", key, id)
		}
		if e != nil {
			tx.Rollback(ctx)
			return
		}
		if tx.Commit(ctx) != nil {
			return
		}
	}
}
func (a *App) ensureLocalAudio(ctx context.Context, id, path, key string) error {
	if _, e := os.Stat(path); e == nil {
		_ = os.Chtimes(path, time.Now(), time.Now())
		return nil
	}
	if a.Objects == nil || key == "" {
		return fmt.Errorf("audio copy unavailable")
	}
	_, e, _ := a.downloads.Do(id, func() (any, error) {
		if _, e := os.Stat(path); e == nil {
			return nil, nil
		}
		call, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		r, e := a.Objects.Get(call, key)
		if e != nil {
			return nil, e
		}
		defer r.Close()
		f, e := os.CreateTemp(a.Cfg.AudioDir, "restore-*.tmp")
		if e != nil {
			return nil, e
		}
		temp := f.Name()
		defer os.Remove(temp)
		n, e := io.Copy(f, io.LimitReader(r, (24<<20)+1))
		ce := f.Close()
		if e != nil {
			return nil, e
		}
		if ce != nil {
			return nil, ce
		}
		if n > 24<<20 {
			return nil, fmt.Errorf("audio too large")
		}
		return nil, os.Rename(temp, path)
	})
	return e
}
func (a *App) expireAudio(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	rows, e := a.DB.Query(ctx, "SELECT id::text FROM audio_assets WHERE expires_at<now() LIMIT 200")
	if e != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		tx, e := a.DB.Begin(ctx)
		if e != nil {
			return
		}
		var path, key string
		e = tx.QueryRow(ctx, "SELECT path,object_key FROM audio_assets WHERE id=$1 AND expires_at<now() FOR UPDATE SKIP LOCKED", id).Scan(&path, &key)
		if e == pgx.ErrNoRows {
			tx.Rollback(ctx)
			continue
		}
		if e != nil {
			tx.Rollback(ctx)
			return
		}
		if a.Objects != nil && key != "" {
			call, cancel := context.WithTimeout(ctx, 10*time.Second)
			e = a.Objects.Delete(call, key)
			cancel()
			if e != nil {
				tx.Rollback(ctx)
				continue
			}
		}
		if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
			tx.Rollback(ctx)
			continue
		}
		if _, e = tx.Exec(ctx, "DELETE FROM audio_assets WHERE id=$1", id); e != nil {
			tx.Rollback(ctx)
			continue
		}
		_ = tx.Commit(ctx)
	}
	// Only evict durable local replicas. User recordings retain their 30-day policy;
	// lesson/TTS objects remain reusable remotely after local cache eviction.
	if a.Objects == nil {
		return
	}
	rows, e = a.DB.Query(ctx, "SELECT path FROM audio_assets WHERE uploaded_at IS NOT NULL")
	if e != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		if rows.Scan(&path) != nil {
			continue
		}
		if info, e := os.Stat(path); e == nil && time.Since(info.ModTime()) > time.Duration(a.Cfg.AudioLocalCacheDays)*24*time.Hour && filepath.Dir(path) == filepath.Clean(a.Cfg.AudioDir) {
			_ = os.Remove(path)
		}
	}
}
