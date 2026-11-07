package main

import (
	"billionmail-core/internal/service/chatbot"
	"billionmail-core/internal/service/database_initialization"
	"billionmail-core/internal/service/notifications"
	"billionmail-core/internal/service/public"
	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/rbac"
	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/whatsappdelivery"
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
	if err := database_initialization.InitWorkerDatabase(ctx); err != nil {
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
	secretStore, err := secrets.NewConfiguredStore(ctx)
	if err != nil {
		panic(err)
	}
	deliveryHandler := whatsappdelivery.Handler{Store: secretStore, APIVersion: public.MustGetDockerEnv("META_GRAPH_API_VERSION", "v23.0")}
	processor := platformqueue.Processor{Consumer: "saas-worker", Handlers: map[string]platformqueue.Handler{
		"platform.schedule.probe":               func(context.Context, platformqueue.Event) error { return nil },
		"identity.email_verification.requested": emailHandler.Handle,
		"identity.password_reset.requested":     emailHandler.Handle,
		"company.invitation.created":            emailHandler.Handle,
		"whatsapp.webhook.received":             chatbot.HandleWebhook,
		"whatsapp.message.send":                 deliveryHandler.Handle,
	}}

	server := asynq.NewServer(redisOpt, asynq.Config{
		ErrorHandler: asynq.ErrorHandlerFunc(func(taskCtx context.Context, task *asynq.Task, failure error) {
			errorCtx, cancel := context.WithTimeout(context.WithoutCancel(taskCtx), 5*time.Second)
			defer cancel()
			if err := platformqueue.RecordTaskFailure(errorCtx, task, failure); err != nil {
				fmt.Fprintln(os.Stderr, "record task failure:", err)
			}
		}),
		Concurrency:     10,
		Queues:          map[string]int{"webhooks": 8, "delivery": 8, "notifications": 4, "default": 2},
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
