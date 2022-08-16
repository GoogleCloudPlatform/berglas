package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GoogleCloudPlatform/berglas/pkg/berglas"
	"github.com/sirupsen/logrus"
	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhlog "github.com/slok/kubewebhook/v2/pkg/log"
	kwhlogrus "github.com/slok/kubewebhook/v2/pkg/log/logrus"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhmutating "github.com/slok/kubewebhook/v2/pkg/webhook/mutating"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// berglasContainer is the default berglas container from which to pull the
	// berglas binary.
	berglasContainer = "ghcr.io/GoogleCloudPlatform/berglas:latest"

	// binVolumeName is the name of the volume where the berglas binary is stored.
	binVolumeName = "berglas-bin"

	// binVolumeMountPath is the mount path where the berglas binary can be found.
	binVolumeMountPath = "/berglas/bin/"
)

// binInitContainer is the container that pulls the berglas binary executable
// into a shared volume mount.
var binInitContainer = corev1.Container{
	Name:            "copy-berglas-bin",
	Image:           berglasContainer,
	ImagePullPolicy: corev1.PullIfNotPresent,
	Command: []string{"sh", "-c",
		fmt.Sprintf("cp /bin/berglas %s", binVolumeMountPath)},
	VolumeMounts: []corev1.VolumeMount{
		{
			Name:      binVolumeName,
			MountPath: binVolumeMountPath,
		},
	},
}

// binVolume is the shared, in-memory volume where the berglas binary lives.
var binVolume = corev1.Volume{
	Name: binVolumeName,
	VolumeSource: corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: corev1.StorageMediumMemory,
		},
	},
}

// binVolumeMount is the shared volume mount where the berglas binary lives.
var binVolumeMount = corev1.VolumeMount{
	Name:      binVolumeName,
	MountPath: binVolumeMountPath,
	ReadOnly:  true,
}

// BerglasMutator is a mutator.
type BerglasMutator struct {
	logger kwhlog.Logger
}

// Mutate implements MutateFunc and provides the top-level entrypoint for object
// mutation.
func (m *BerglasMutator) Mutate(ctx context.Context, ar *kwhmodel.AdmissionReview, obj metav1.Object) (*kwhmutating.MutatorResult, error) {
	m.logger.Infof("calling mutate")

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return &kwhmutating.MutatorResult{
			Warnings: []string{fmt.Sprintf("incoming resource is not a Pod (%T)", pod)},
		}, nil
	}

	mutated := false
	for i, c := range pod.Spec.InitContainers {
		c, didMutate := m.mutateContainer(ctx, &c)
		if didMutate {
			mutated = true
			pod.Spec.InitContainers[i] = *c
		}
	}

	for i, c := range pod.Spec.Containers {
		c, didMutate := m.mutateContainer(ctx, &c)
		if didMutate {
			mutated = true
			pod.Spec.Containers[i] = *c
		}
	}

	// If any of the containers requested berglas secrets, mount the shared volume
	// and ensure the berglas binary is available via an init container.
	if mutated {
		pod.Spec.Volumes = append(pod.Spec.Volumes, binVolume)
		pod.Spec.InitContainers = append([]corev1.Container{binInitContainer},
			pod.Spec.InitContainers...)
	}

	return &kwhmutating.MutatorResult{
		MutatedObject: pod,
	}, nil
}

// mutateContainer mutates the given container, updating the volume mounts and
// command if it contains berglas references.
func (m *BerglasMutator) mutateContainer(_ context.Context, c *corev1.Container) (*corev1.Container, bool) {
	// Ignore if there are no berglas references in the container.
	if !m.hasBerglasReferences(c.Env) {
		return c, false
	}

	// Berglas prepends the command from the podspec. If there's no command in the
	// podspec, there's nothing to append. Note: this is the command in the
	// podspec, not a CMD or ENTRYPOINT in a Dockerfile.
	if len(c.Command) == 0 {
		m.logger.Warningf("cannot apply berglas to %s: container spec does not define a command", c.Name)
		return c, false
	}

	// Add the shared volume mount
	c.VolumeMounts = append(c.VolumeMounts, binVolumeMount)

	// Prepend the command with berglas exec --
	original := append(c.Command, c.Args...)
	c.Command = []string{binVolumeMountPath + "berglas"}
	c.Args = append([]string{"exec", "--"}, original...)

	return c, true
}

// hasBerglasReferences parses the environment and returns true if any of the
// environment variables includes a berglas reference.
func (m *BerglasMutator) hasBerglasReferences(env []corev1.EnvVar) bool {
	for _, e := range env {
		if berglas.IsReference(e.Value) {
			return true
		}
	}
	return false
}

// webhookHandler is the http.Handler that responds to webhooks
func webhookHandler() (http.Handler, error) {
	entry := logrus.NewEntry(logrus.New())
	entry.Logger.SetLevel(logrus.DebugLevel)
	logger := kwhlogrus.NewLogrus(entry)

	mutator := &BerglasMutator{logger: logger}

	mcfg := kwhmutating.WebhookConfig{
		ID:      "berglasSecrets",
		Obj:     &corev1.Pod{},
		Mutator: mutator,
		Logger:  logger,
	}

	// Create the wrapping webhook
	wh, err := kwhmutating.NewWebhook(mcfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create mutating webhook: %w", err)
	}

	// Get the handler for our webhook.
	whhandler, err := kwhhttp.HandlerFor(kwhhttp.HandlerConfig{
		Webhook: wh,
		Logger:  logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create mutating webhook handler: %w", err)
	}
	return whhandler, nil
}

func logAt(lvl, msg string, args ...any) {
	body := map[string]any{
		"time":     time.Now().UTC().Format(time.RFC3339),
		"severity": lvl,
		"message":  fmt.Sprintf(msg, args...),
	}

	payload, err := json.Marshal(body)
	if err != nil {
		panic(fmt.Sprintf("failed to make JSON error message: %s", err))
	}
	fmt.Fprintln(os.Stderr, string(payload))
}

func logInfo(msg string, args ...any) {
	logAt("INFO", msg, args...)
}

func logError(msg string, args ...any) {
	logAt("ERROR", msg, args...)
}

func realMain() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler, err := webhookHandler()
	if err != nil {
		return fmt.Errorf("server failed to start: %w", err)
	}

	srv := http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	logInfo("server is listening on " + port)

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	<-stopCh

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server failed to shutdown: %w", err)
	}

	// Wait for shutdown
	if err := <-errCh; err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func main() {
	if err := realMain(); err != nil {
		logError(err.Error())
		os.Exit(1)
	}

	logInfo("server successfully stopped")
}
