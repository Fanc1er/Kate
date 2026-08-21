// Package app 承载 Master 进程装配与启动逻辑。
package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	seed := service.NewSeedService(gdb, cfg.SuperAdminUser, cfg.SuperAdminPass)
	asset := service.NewAssetService(gdb, cache, audit)
	assessor := service.NewResultAssessor(gdb)
	task := service.NewTaskService(gdb, audit, assessor)
	policy := service.NewPolicyService(gdb, audit)
	hub := service.NewHub()
	evidence := service.NewEvidenceService(gdb, store, cfg.EvidenceTTL)
	worker := service.NewWorkerService(gdb, task, evidence, hub, cfg.StormLimitHour)
	triage := service.NewTriageService(gdb, audit, evidence)
	dashboard := service.NewDashboardService(gdb)
	member := service.NewMemberService(gdb, audit, mail)
	report := service.NewReportService(gdb)
	sec := middleware.NewSecurity(gdb, tokens)

	if _, pwd, err := seed.EnsureSuperAdmin(); err != nil {
		log.Fatalf("init super admin: %v", err)
	} else if pwd != "" {
		log.Printf("super_admin 已创建：%s，临时密码：%s（仅首次打印）", cfg.SuperAdminUser, pwd)
	}
	if err := seed.EnsureDefaults(); err != nil {
		log.Fatalf("init defaults: %v", err)
	}

	sched := service.NewMasterScheduler(gdb, cache, evidence)
	cron := service.NewCronScheduler(gdb, task)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Run(ctx)
	go cron.Run(ctx)

	engine := routes.Setup(&routes.Deps{
		DB:        gdb,
		Auth:      auth,
		Seed:      seed,
		Asset:     asset,
		Task:      task,
		Policy:    policy,
		Triage:    triage,
		Evidence:  evidence,
		Worker:    worker,
		Dashboard: dashboard,
		Member:    member,
		Report:    report,
		Tokens:    tokens,
		Security:  sec,
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
