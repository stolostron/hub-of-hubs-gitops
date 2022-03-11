// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/go-logr/logr"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/authorizer"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/controller"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db"
	"github.com/stolostron/hub-of-hubs-nonk8s-gitops/pkg/db/postgresql"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	metricsHost                     = "0.0.0.0"
	metricsPort               int32 = 8965
	envVarControllerNamespace       = "POD_NAMESPACE"
	envVarSyncInterval              = "SYNC_INTERVAL"
	envVarGitStorageDirPath         = "SUBSCRIPTION_GIT_STORAGE_DIR_PATH"
	leaderElectionLockName          = "hub-of-hubs-nonk8s-gitops-lock"
)

var errEnvVarNotFound = errors.New("environment variable not found")

func printVersion(log logr.Logger) {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func readEnvVars() (string, time.Duration, error) {
	leaderElectionNamespace, found := os.LookupEnv(envVarControllerNamespace)
	if !found {
		return "", 0, fmt.Errorf("%w: %s", errEnvVarNotFound, envVarControllerNamespace)
	}

	syncIntervalString, found := os.LookupEnv(envVarSyncInterval)
	if !found {
		return "", 0, fmt.Errorf("%w: %s", errEnvVarNotFound, envVarSyncInterval)
	}

	syncInterval, err := time.ParseDuration(syncIntervalString)
	if err != nil {
		return "", 0, fmt.Errorf("the environment var %s is not a valid duration - %w",
			envVarSyncInterval, err)
	}

	return leaderElectionNamespace, syncInterval, nil
}

func getGitStorageDir() (string, error) {
	gitStorageDirPath, found := os.LookupEnv(envVarGitStorageDirPath)
	if !found {
		return "", fmt.Errorf("%w: %s", errEnvVarNotFound, envVarControllerNamespace)
	}

	if _, err := os.Open(gitStorageDirPath); err != nil {
		return "", fmt.Errorf("%w : %s", err, "failed to open directory")
	}

	return gitStorageDirPath, nil
}

// function to handle defers with exit, see https://stackoverflow.com/a/27629493/553720.
func doMain() int {
	pflag.CommandLine.AddFlagSet(zap.FlagSet())
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(zap.Logger())
	log := ctrl.Log.WithName("cmd")

	printVersion(log)

	leaderElectionNamespace, syncInterval, err := readEnvVars()
	if err != nil {
		log.Error(err, "initialization error")
		return 1
	}

	gitStorageDirPath, err := getGitStorageDir()
	if err != nil {
		log.Error(err, "initialization error")
		return 1
	}

	// db layer initialization
	postgreSQL, err := postgresql.NewPostgreSQL()
	if err != nil {
		log.Error(err, "initialization error", "failed to initialize", "PostgreSQL")
		return 1
	}

	defer postgreSQL.Stop()

	rbacAuthorizer, err := authorizer.NewHubOfHubsAuthorizer(postgreSQL)
	if err != nil {
		log.Error(err, "initialization error", "failed to initialize", "Authorizer")
		return 1
	}

	mgr, err := createManager(leaderElectionNamespace, gitStorageDirPath, postgreSQL, rbacAuthorizer, syncInterval)
	if err != nil {
		log.Error(err, "Failed to create manager")
		return 1
	}

	log.Info("Starting the Cmd.")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		return 1
	}

	return 0
}

func createManager(leaderElectionNamespace string, gitStorageDirPath string, specDB db.SpecDB,
	authorizer authorizer.Authorizer, syncInterval time.Duration,
) (ctrl.Manager, error) {
	options := ctrl.Options{
		MetricsBindAddress:      fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		LeaderElection:          true,
		LeaderElectionID:        leaderElectionLockName,
		LeaderElectionNamespace: leaderElectionNamespace,
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errEnvVarNotFound, envVarControllerNamespace)
	}

	if err := controller.AddGitStorageWalker(mgr, gitStorageDirPath, specDB, authorizer, syncInterval); err != nil {
		return nil, fmt.Errorf("failed to add db syncers: %w", err)
	}

	return mgr, nil
}

func main() {
	os.Exit(doMain())
}
