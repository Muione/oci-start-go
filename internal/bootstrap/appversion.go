// Package bootstrap performs startup initialization parity with Java's Init.run
// (CommandLineRunner) + VersionCheckTask.init. Per D3, initDeviceRegistration
// (phone-home to the CF Worker) is NOT ported. See SPEC §11.
package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/config"
	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/Muione/oci-start-go/internal/util/version"
	"github.com/rs/zerolog"
)

// AppVersion seeds the single app_version row if absent and reconciles its
// current/latest version from config, mirroring VersionCheckTask.init +
// Init.checkAppVersion. Deploy type is detected at runtime (/.dockerenv).
func AppVersion(ctx context.Context, store *db.Store, cfg *config.Config, logger zerolog.Logger) error {
	q := repo.New(store.Write)
	now := time.Now().Format("2006-01-02 15:04:05")
	deployType := version.DetectDeployType(cfg.Deploy.Type)

	// Seed if absent (INSERT OR IGNORE on id=1).
	if err := q.SeedAppVersion(ctx, repo.SeedAppVersionParams{
		CurrentVersion: cfg.Oci.Version,
		LatestVersion:  cfg.Oci.SshVersion,
		DeployType:     deployType,
		CreateTime:     now,
		UpdateTime:     now,
	}); err != nil {
		return fmt.Errorf("seed app_version: %w", err)
	}

	// Reconcile current/latest from config.
	if err := q.UpdateAppVersion(ctx, repo.UpdateAppVersionParams{
		CurrentVersion: cfg.Oci.Version,
		LatestVersion:  cfg.Oci.SshVersion,
		UpdateTime:     now,
	}); err != nil {
		return fmt.Errorf("update app_version: %w", err)
	}

	ver, err := repo.New(store.Read).GetAppVersion(ctx)
	if err != nil {
		return fmt.Errorf("read app_version: %w", err)
	}
	logger.Info().
		Str("current", ver.CurrentVersion).
		Str("latest", ver.LatestVersion).
		Str("deploy", ver.DeployType).
		Msg("app version ready")
	return nil
}
