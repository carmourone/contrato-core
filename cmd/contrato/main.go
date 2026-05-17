package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"contrato/internal/app"
	"contrato/internal/config"
	"contrato/internal/migrate"
	"contrato/internal/modelio"
	pg "contrato/internal/storage/drivers/postgres"
)

func main() {
	importPath := flag.String("import", "", "Import a model bundle JSON file and exit")
	exportPath := flag.String("export", "", "Export latest enabled model bundle to JSON and exit")
	tenant := flag.String("tenant", "", "Tenant name (required for export; optional for import if bundle includes it)")
	enable := flag.Bool("enable", false, "When importing, mark created model_version as enabled")
	note := flag.String("note", "", "Optional change note override on import")
	withContracts := flag.Bool("with-contracts", false, "Include contracts in import/export bundles")
	lintPath := flag.String("lint", "", "Lint a model bundle JSON file and exit")
	bootstrapPath := flag.String("bootstrap", "", "Bootstrap tenant/model with a core bundle JSON file and exit")
	flag.Parse()

	cfg := config.FromEnv()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *lintPath != "" {
		raw, err := os.ReadFile(*lintPath)
		if err != nil {
			log.Fatalf("lint read failed: %v", err)
		}
		var b modelio.Bundle
		if err := json.Unmarshal(raw, &b); err != nil {
			log.Fatalf("lint parse failed: %v", err)
		}
		rep := modelio.LintBundle(b, modelio.LintModeEnabledModel)
		if !rep.OK {
			log.Fatalf("lint failed:\n%s", rep.JSON())
		}
		log.Printf("lint ok")
		return
	}

	if *importPath != "" || *exportPath != "" || *bootstrapPath != "" {
		drv := pg.Driver{Migrate: migrate.Run}
		store, err := drv.Open(ctx, map[string]any{"dsn": cfg.PostgresDSN})
		if err != nil {
			log.Fatalf("failed to open store: %v", err)
		}
		defer store.Close()

		swd, ok := store.(modelio.StoreWithDB)
		if !ok {
			log.Fatalf("store does not support export/import")
		}

		if *bootstrapPath != "" {
			raw, err := os.ReadFile(*bootstrapPath)
			if err != nil {
				log.Fatalf("bootstrap read failed: %v", err)
			}
			var b modelio.Bundle
			if err := json.Unmarshal(raw, &b); err != nil {
				log.Fatalf("bootstrap parse failed: %v", err)
			}
			rep := modelio.LintBundle(b, modelio.LintModeBootstrap)
			if !rep.OK {
				log.Fatalf("bootstrap lint failed:\n%s", rep.JSON())
			}
			err = modelio.ImportBundle(ctx, swd, modelio.Options{
				TenantName: *tenant, InputPath: *bootstrapPath,
				EnableImported: true, ChangeNote: *note, WithContracts: *withContracts,
			})
			if err != nil {
				log.Fatalf("bootstrap import failed: %v", err)
			}
			log.Printf("bootstrapped tenant/model from %s", *bootstrapPath)
			return
		}

		if *exportPath != "" {
			err := modelio.ExportLatestEnabled(ctx, swd, modelio.Options{
				TenantName: *tenant, OutputPath: *exportPath, WithContracts: *withContracts,
			})
			if err != nil {
				log.Fatalf("export failed: %v", err)
			}
			log.Printf("exported model bundle to %s", *exportPath)
			return
		}

		if *importPath != "" {
			err := modelio.ImportBundle(ctx, swd, modelio.Options{
				TenantName: *tenant, InputPath: *importPath,
				EnableImported: *enable, ChangeNote: *note, WithContracts: *withContracts,
			})
			if err != nil {
				log.Fatalf("import failed: %v", err)
			}
			log.Printf("imported model bundle from %s", *importPath)
			return
		}
	}

	a, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to init app: %v", err)
	}
	defer a.Close()

	srv := a.HTTPServer()

	go func() {
		log.Printf("Contrato listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("http server stopped: %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	cancel()
}
