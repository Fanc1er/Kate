// Package app 承载 Master 进程装配与启动逻辑。
package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/license"
	"github.com/Fanc1er/Kate/backend/internal/master/middleware"
	"github.com/Fanc1er/Kate/backend/internal/master/routes"
	"github.com/Fanc1er/Kate/backend/internal/master/service"
	"github.com/Fanc1er/Kate/backend/pkg/badger"
	"github.com/Fanc1er/Kate/backend/pkg/config"
	"github.com/Fanc1er/Kate/backend/pkg/db"
	"github.com/Fanc1er/Kate/backend/pkg/storage"
)

// Run 启动 Master 服务并阻塞直到收到退出信号。
func Run() {
	cfg := config.Load()

	gdb, err := db.Init(cfg.DBPath)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	cache, err := badger.Open(badger.DataDir(cfg.DataDir))
	if err != nil {
		log.Fatalf("init badger: %v", err)
	}
	defer cache.Close()
	store, err := storage.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("init evidence storage: %v", err)
	}

	logger := func(level, msg string, fields map[string]any) {
		log.Printf("[%s] %s %v", level, msg, fields)
	}
	tokens := service.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL, cfg.RefreshTTL)
	audit := service.NewAuditWriter(gdb)
	mail := service.NewMailService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPFrom, cfg.MailCodeTTL)
	auth := service.NewAuthService(gdb, tokens, mail)
	seed := service.NewSeedService(gdb, cfg.AdminUser, cfg.AdminPass)

	licensePath := cfg.LicensePath
	if licensePath == "" {
		licensePath = filepath.Join(cfg.DataDir, "license.lic")
	}
	saltPath := filepath.Join(cfg.DataDir, "machine.salt")
	lic, err := license.NewManager(licensePath, saltPath)
	if err != nil {
		log.Fatalf("init license: %v", err)
	}
	if err := lic.Load(); err != nil {
		log.Fatalf("load license: %v", err)
	}

	asset := service.NewAssetService(gdb, cache, audit, lic)
	assessor := service.NewResultAssessor(gdb)
	task := service.NewTaskService(gdb, audit, assessor)
	policy := service.NewPolicyService(gdb, audit)
	hub := service.NewHub()
	evidence := service.NewEvidenceService(gdb, store, cfg.EvidenceTTL)
	worker := service.NewWorkerService(gdb, task, evidence, hub, lic, cfg.StormLimitHour)
	triage := service.NewTriageService(gdb, audit, evidence)
	dashboard := service.NewDashboardService(gdb)
	availability := service.NewAvailabilityService(gdb, task)
	member := service.NewMemberService(gdb, audit, mail)
	intel := service.NewIntelService(gdb, audit)
	report := service.NewReportService(gdb)

	sec := middleware.NewSecurity(gdb, tokens, lic)

	if _, pwd, err := seed.EnsureAdmin(); err != nil {
		log.Fatalf("init admin: %v", err)
	} else if pwd != "" {
		log.Printf("admin 已创建：%s，临时密码：%s（仅首次打印）", cfg.AdminUser, pwd)
	}
	if err := seed.EnsureDefaults(); err != nil {
		log.Fatalf("init defaults: %v", err)
	}

	sched := service.NewMasterScheduler(gdb, cache, evidence)
	cron := service.NewCronScheduler(gdb, task, lic)
	probe := service.NewProbeScheduler(gdb, task, lic)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Run(ctx)
	go cron.Run(ctx)
	go probe.Run(ctx)

	engine := routes.Setup(&routes.Deps{
		DB:        gdb,
		Auth:      auth,
		Seed:      seed,
		Asset:     asset,
		Task:      task,
		Policy:    policy,
		Triage:    triage,
		Evidence:  evidence,
		Worker:       worker,
		Dashboard:    dashboard,
		Availability: availability,
		Member:       member,
		Intel:        intel,
		Report:    report,
		Tokens:    tokens,
		Security:  sec,
		License:   lic,
		Hub:       hub,
		Logger:    logger,
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: engine}
	go func() {
		log.Printf("cinsight-master listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	log.Println("shutting down master...")
	ctxShut, cancelShut := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShut()
	_ = srv.Shutdown(ctxShut)
	cancel()
}
