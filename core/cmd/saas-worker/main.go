package main

import (
	"billionmail-core/internal/service/database_initialization"
	"billionmail-core/internal/service/notifications"
	"billionmail-core/internal/service/public"
	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/rbac"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := database_initialization.InitDatabase(); err != nil {
		panic(err)
	}
	redisOpt := platformqueue.RedisClientOpt()
	client := asynq.NewClient(redisOpt)
	defer client.Close()

	var mailer notifications.Mailer
	fromEmail := public.MustGetDockerEnv("SAAS_FROM_EMAIL", "noreply@localhost")
	if public.MustGetDockerEnv("SAAS_MAIL_DRIVER", "smtp") == "ses" {
		awsConfig, configErr := awsconfig.LoadDefaultConfig(ctx)
		if configErr != nil {
			panic(configErr)
		}
		mailer = notifications.SESMailer{Client: sesv2.NewFromConfig(awsConfig), From: fromEmail}
	} else {
		mailer = notifications.SMTPMailer{
			Address: public.MustGetDockerEnv("SAAS_SMTP_ADDR", "postfix:25"),
			From:    fromEmail,
		}
	}
	emailHandler := notifications.IdentityEmailHandler{Mailer: mailer, TokenSecret: rbac.JWT().Secret}
	processor := platformqueue.Processor{Consumer: "notification-worker", Handlers: map[string]platformqueue.Handler{
		"identity.email_verification.requested": emailHandler.Handle,
		"identity.password_reset.requested":     emailHandler.Handle,
		"company.invitation.created":            emailHandler.Handle,
	}}

	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:     10,
		Queues:          map[string]int{"notifications": 8, "default": 2},
		ShutdownTimeout: 30 * time.Second,
	})
	mux := asynq.NewServeMux()
	mux.HandleFunc(platformqueue.OutboxTaskType, processor.HandleTask)

	workerID, _ := os.Hostname()
	workerID += "-" + uuid.NewString()[:8]
	dispatcher := platformqueue.Dispatcher{Publisher: platformqueue.AsynqPublisher{Client: client}, WorkerID: workerID}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := dispatcher.Dispatch(ctx); err != nil && ctx.Err() == nil {
					fmt.Fprintln(os.Stderr, "outbox dispatch:", err)
				}
			}
		}
	}()
	go func() { <-ctx.Done(); server.Shutdown() }()
	if err := server.Run(mux); err != nil {
		panic(err)
	}
}
