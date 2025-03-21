package main

import (
	"context"
	"fmt"
	"time"

	"github.com/glasskube/distr/api"
	"github.com/glasskube/distr/internal/agentauth"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	helmEnvSettings       = cli.New()
	helmActionConfigCache = make(map[string]*action.Configuration)
)

func GetHelmActionConfig(
	ctx context.Context,
	namespace string,
	deployment *api.AgentDeployment,
) (*action.Configuration, error) {
	if cfg, ok := helmActionConfigCache[namespace]; ok {
		return cfg, nil
	}

	var cfg action.Configuration
	if deployment != nil {
		if authorizer, err := agentauth.EnsureAuth(ctx, *deployment); err != nil {
			return nil, err
		} else if rc, err := registry.NewClient(registry.ClientOptAuthorizer(authorizer)); err != nil {
			return nil, err
		} else {
			cfg.RegistryClient = rc
		}
	}
	if err := cfg.Init(
		k8sConfigFlags,
		namespace,
		"secret",
		func(format string, v ...interface{}) { logger.Sugar().Debugf(format, v...) },
	); err != nil {
		return nil, err
	} else {
		return &cfg, nil
	}
}

func GetLatestHelmRelease(
	ctx context.Context,
	namespace string,
	deployment api.KubernetesAgentDeployment,
) (*release.Release, error) {
	cfg, err := GetHelmActionConfig(ctx, namespace, &deployment.AgentDeployment)
	if err != nil {
		return nil, err
	}
	historyAction := action.NewHistory(cfg)
	if releases, err := historyAction.Run(deployment.ReleaseName); err != nil {
		return nil, err
	} else {
		return releases[len(releases)-1], nil
	}
}

func RunHelmPreflight(
	action *action.ChartPathOptions,
	deployment api.KubernetesAgentDeployment,
) (*chart.Chart, error) {
	chartName := deployment.ChartName
	action.Version = deployment.ChartVersion
	if registry.IsOCI(deployment.ChartUrl) {
		chartName = deployment.ChartUrl
	} else {
		action.RepoURL = deployment.ChartUrl
	}
	if chartPath, err := action.LocateChart(chartName, helmEnvSettings); err != nil {
		return nil, fmt.Errorf("could not locate chart: %w", err)
	} else if chart, err := loader.Load(chartPath); err != nil {
		return nil, fmt.Errorf("chart loading failed: %w", err)
	} else {
		return chart, nil
	}
}

func RunHelmInstall(
	ctx context.Context,
	namespace string,
	deployment api.KubernetesAgentDeployment,
) (*AgentDeployment, error) {
	config, err := GetHelmActionConfig(ctx, namespace, &deployment.AgentDeployment)
	if err != nil {
		return nil, err
	}

	installAction := action.NewInstall(config)
	installAction.ReleaseName = deployment.ReleaseName

	installAction.Timeout = 5 * time.Minute
	installAction.Wait = true
	installAction.Atomic = true
	installAction.Namespace = namespace
	if chart, err := RunHelmPreflight(&installAction.ChartPathOptions, deployment); err != nil {
		return nil, fmt.Errorf("helm preflight failed: %w", err)
	} else if release, err := installAction.RunWithContext(ctx, chart, deployment.Values); err != nil {
		return nil, fmt.Errorf("helm install failed: %w", err)
	} else {
		return &AgentDeployment{
			ReleaseName:  release.Name,
			HelmRevision: release.Version,
			RevisionID:   deployment.RevisionID,
		}, nil
	}
}

func RunHelmUpgrade(
	ctx context.Context,
	namespace string,
	deployment api.KubernetesAgentDeployment,
) (*AgentDeployment, error) {
	cfg, err := GetHelmActionConfig(ctx, namespace, &deployment.AgentDeployment)
	if err != nil {
		return nil, err
	}

	upgradeAction := action.NewUpgrade(cfg)
	upgradeAction.CleanupOnFail = true

	upgradeAction.Timeout = 5 * time.Minute
	upgradeAction.Wait = true
	upgradeAction.Atomic = true
	upgradeAction.Namespace = namespace
	if chart, err := RunHelmPreflight(&upgradeAction.ChartPathOptions, deployment); err != nil {
		return nil, fmt.Errorf("helm preflight failed: %w", err)
	} else if release, err := upgradeAction.RunWithContext(
		ctx, deployment.ReleaseName, chart, deployment.Values); err != nil {
		return nil, fmt.Errorf("helm upgrade failed: %w", err)
	} else {
		return &AgentDeployment{
			ReleaseName:  release.Name,
			HelmRevision: release.Version,
			RevisionID:   deployment.RevisionID,
		}, nil
	}
}

func RunHelmUninstall(ctx context.Context, namespace, releaseName string) error {
	config, err := GetHelmActionConfig(ctx, namespace, nil)
	if err != nil {
		return err
	}

	uninstallAction := action.NewUninstall(config)
	uninstallAction.Timeout = 5 * time.Minute
	uninstallAction.Wait = true
	uninstallAction.IgnoreNotFound = true
	if _, err := uninstallAction.Run(releaseName); err != nil {
		return fmt.Errorf("helm uninstall failed: %w", err)
	}
	return nil
}

func GetHelmManifest(
	ctx context.Context,
	namespace string,
	deployment api.KubernetesAgentDeployment,
) ([]*unstructured.Unstructured, error) {
	cfg, err := GetHelmActionConfig(ctx, namespace, &deployment.AgentDeployment)
	if err != nil {
		return nil, err
	}
	getAction := action.NewGet(cfg)
	if release, err := getAction.Run(deployment.ReleaseName); err != nil {
		return nil, err
	} else {
		// decode the release manifests which is represented as multi-document YAML
		return DecodeResourceYaml([]byte(release.Manifest))
	}
}
