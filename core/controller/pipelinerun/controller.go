package pipelinerun

import (
	"context"
	"fmt"
	"strings"

	"g.hz.netease.com/horizon/core/common"
	"g.hz.netease.com/horizon/lib/q"
	appmanager "g.hz.netease.com/horizon/pkg/application/manager"
	"g.hz.netease.com/horizon/pkg/cluster/code"
	"g.hz.netease.com/horizon/pkg/cluster/gitrepo"
	clustermanager "g.hz.netease.com/horizon/pkg/cluster/manager"
	clustermodels "g.hz.netease.com/horizon/pkg/cluster/models"
	"g.hz.netease.com/horizon/pkg/cluster/tekton/factory"
	"g.hz.netease.com/horizon/pkg/cluster/tekton/log"
	envmanager "g.hz.netease.com/horizon/pkg/environment/manager"
	prmanager "g.hz.netease.com/horizon/pkg/pipelinerun/manager"
	"g.hz.netease.com/horizon/pkg/pipelinerun/models"
	prmodels "g.hz.netease.com/horizon/pkg/pipelinerun/models"
	usermanager "g.hz.netease.com/horizon/pkg/user/manager"
	"g.hz.netease.com/horizon/pkg/util/errors"
	"g.hz.netease.com/horizon/pkg/util/wlog"
)

type Controller interface {
	GetPipelinerunLog(ctx context.Context, prID uint) (*Log, error)
	GetClusterLatestLog(ctx context.Context, clusterID uint) (*Log, error)
	GetDiff(ctx context.Context, pipelinerunID uint) (*GetDiffResponse, error)
	Get(ctx context.Context, pipelinerunID uint) (*PipelineBasic, error)
	List(ctx context.Context, clusterID uint, query q.Query) (int, []*PipelineBasic, error)
}

type controller struct {
	pipelinerunMgr prmanager.Manager
	applicationMgr appmanager.Manager
	clusterMgr     clustermanager.Manager
	envMgr         envmanager.Manager
	tektonFty      factory.Factory
	commitGetter   code.CommitGetter
	clusterGitRepo gitrepo.ClusterGitRepo
	userManager    usermanager.Manager
}

var _ Controller = (*controller)(nil)

func NewController(tektonFty factory.Factory, codeGetter code.CommitGetter,
	clusterRepo gitrepo.ClusterGitRepo) Controller {
	return &controller{
		pipelinerunMgr: prmanager.Mgr,
		clusterMgr:     clustermanager.Mgr,
		envMgr:         envmanager.Mgr,
		tektonFty:      tektonFty,
		commitGetter:   codeGetter,
		applicationMgr: appmanager.Mgr,
		clusterGitRepo: clusterRepo,
		userManager:    usermanager.Mgr,
	}
}

type Log struct {
	LogChannel <-chan log.Log
	ErrChannel <-chan error

	LogBytes []byte
}

func (c *controller) GetPipelinerunLog(ctx context.Context, prID uint) (_ *Log, err error) {
	const op = "pipelinerun controller: get pipelinerun log"
	defer wlog.Start(ctx, op).Stop(func() string { return wlog.ByErr(err) })

	pr, err := c.pipelinerunMgr.GetByID(ctx, prID)
	if err != nil {
		return nil, errors.E(op, err)
	}

	cluster, err := c.clusterMgr.GetByID(ctx, pr.ClusterID)
	if err != nil {
		return nil, errors.E(op, err)
	}

	er, err := c.envMgr.GetEnvironmentRegionByID(ctx, cluster.EnvironmentRegionID)
	if err != nil {
		return nil, errors.E(op, err)
	}

	// only builddeploy have logs
	if pr.Action != prmodels.ActionBuildDeploy {
		return nil, errors.E(op, fmt.Errorf("%v action has no log", pr.Action))
	}

	return c.getPipelinerunLog(ctx, pr, cluster, er.EnvironmentName)
}

func (c *controller) GetClusterLatestLog(ctx context.Context, clusterID uint) (_ *Log, err error) {
	const op = "pipelinerun controller: get cluster latest log"
	defer wlog.Start(ctx, op).Stop(func() string { return wlog.ByErr(err) })

	pr, err := c.pipelinerunMgr.GetLatestByClusterIDAndAction(ctx, clusterID, prmodels.ActionBuildDeploy)
	if err != nil {
		return nil, errors.E(op, err)
	}
	if pr == nil {
		return nil, errors.E(op, fmt.Errorf("no builddeploy pipelinerun"))
	}

	cluster, err := c.clusterMgr.GetByID(ctx, clusterID)
	if err != nil {
		return nil, errors.E(op, err)
	}
	er, err := c.envMgr.GetEnvironmentRegionByID(ctx, cluster.EnvironmentRegionID)
	if err != nil {
		return nil, errors.E(op, err)
	}
	return c.getPipelinerunLog(ctx, pr, cluster, er.EnvironmentName)
}

func (c *controller) getPipelinerunLog(ctx context.Context, pr *prmodels.Pipelinerun, cluster *clustermodels.Cluster,
	environment string) (_ *Log, err error) {
	const op = "pipeline controller: get pipelinerun log"
	defer wlog.Start(ctx, op).Stop(func() string { return wlog.ByErr(err) })

	// if pr.PrObject is empty, get pipelinerun log in k8s
	if pr.PrObject == "" {
		tektonClient, err := c.tektonFty.GetTekton(environment)
		if err != nil {
			return nil, errors.E(op, err)
		}

		logCh, errCh, err := tektonClient.GetPipelineRunLogByID(ctx, cluster.Name, cluster.ID, pr.ID)
		if err != nil {
			return nil, errors.E(op, err)
		}
		return &Log{
			LogChannel: logCh,
			ErrChannel: errCh,
		}, nil
	}

	// else, get log from s3
	tektonCollector, err := c.tektonFty.GetTektonCollector(environment)
	if err != nil {
		return nil, errors.E(op, err)
	}
	logBytes, err := tektonCollector.GetPipelineRunLog(ctx, pr.LogObject)
	if err != nil {
		return nil, errors.E(op, err)
	}
	return &Log{
		LogBytes: logBytes,
	}, nil
}

func (c *controller) GetDiff(ctx context.Context, pipelinerunID uint) (_ *GetDiffResponse, err error) {
	const op = "pipelinerun controller: get pipelinerun diff"
	defer wlog.Start(ctx, op).Stop(func() string { return wlog.ByErr(err) })

	// 1. get pipeline
	pipelinerun, err := c.pipelinerunMgr.GetByID(ctx, pipelinerunID)
	if err != nil {
		return nil, err
	}

	// 2. get cluster and application
	cluster, err := c.clusterMgr.GetByID(ctx, pipelinerun.ClusterID)
	if err != nil {
		return nil, err
	}
	application, err := c.applicationMgr.GetByID(ctx, cluster.ApplicationID)
	if err != nil {
		return nil, err
	}

	// 3. get code diff
	var codeDiff *CodeInfo
	if pipelinerun.GitURL != "" && pipelinerun.GitCommit != "" &&
		pipelinerun.GitBranch != "" {
		commit, err := c.commitGetter.GetCommit(ctx, pipelinerun.GitURL, nil, &pipelinerun.GitCommit)
		if err != nil {
			return nil, err
		}
		var historyLink string
		if strings.HasPrefix(pipelinerun.GitURL, common.InternalGitSSHPrefix) {
			httpURL := common.InternalSSHToHTTPURL(pipelinerun.GitURL)
			historyLink = httpURL + common.CommitHistoryMiddle + pipelinerun.GitCommit
		}
		codeDiff = &CodeInfo{
			Branch:    pipelinerun.GitBranch,
			CommitID:  pipelinerun.GitCommit,
			CommitMsg: commit.Message,
			Link:      historyLink,
		}
	}

	// 4. get config diff
	var configDiff *ConfigDiff
	if pipelinerun.ConfigCommit != "" && pipelinerun.LastConfigCommit != "" {
		diff, err := c.clusterGitRepo.CompareConfig(ctx, application.Name, cluster.Name,
			&pipelinerun.LastConfigCommit, &pipelinerun.ConfigCommit)
		if err != nil {
			return nil, err
		}
		configDiff = &ConfigDiff{
			From: pipelinerun.LastConfigCommit,
			To:   pipelinerun.ConfigCommit,
			Diff: diff,
		}
	}

	return &GetDiffResponse{
		CodeInfo:   codeDiff,
		ConfigDiff: configDiff,
	}, nil
}

func (c *controller) Get(ctx context.Context, pipelineID uint) (_ *PipelineBasic, err error) {
	const op = "pipelinerun controller: get pipelinerun basic"
	defer wlog.Start(ctx, op).Stop(func() string { return wlog.ByErr(err) })

	pipelinerun, err := c.pipelinerunMgr.GetByID(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	return c.ofPipelineBasic(ctx, pipelinerun)
}

func (c *controller) List(ctx context.Context,
	clusterID uint, query q.Query) (_ int, _ []*PipelineBasic, err error) {
	const op = "pipelinerun controller: list pipelinerun"
	defer wlog.Start(ctx, op).Stop(func() string { return wlog.ByErr(err) })

	totalCount, pipelineruns, err := c.pipelinerunMgr.GetByClusterID(ctx, clusterID, query)
	if err != nil {
		return 0, nil, err
	}

	pipelineBasics, err := c.ofPipelineBasics(ctx, pipelineruns)
	if err != nil {
		return 0, nil, err
	}
	return totalCount, pipelineBasics, nil
}

func (c *controller) ofPipelineBasic(ctx context.Context, pr *models.Pipelinerun) (*PipelineBasic, error) {
	user, err := c.userManager.GetUserByID(ctx, pr.CreatedBy)
	if err != nil {
		return nil, err
	}
	return &PipelineBasic{
		ID:               pr.ID,
		Title:            pr.Title,
		Description:      pr.Description,
		Action:           pr.Action,
		Status:           pr.Status,
		GitURL:           pr.GitURL,
		GitBranch:        pr.GitBranch,
		GitCommit:        pr.GitCommit,
		ImageURL:         pr.ImageURL,
		LastConfigCommit: pr.LastConfigCommit,
		ConfigCommit:     pr.ConfigCommit,
		StartedAt:        pr.StartedAt,
		FinishedAt:       pr.FinishedAt,
		CreatedBy: UserInfo{
			UserID:   pr.CreatedBy,
			UserName: user.Name,
		},
	}, nil
}

func (c *controller) ofPipelineBasics(ctx context.Context, prs []*models.Pipelinerun) ([]*PipelineBasic, error) {
	var pipelineBasics []*PipelineBasic
	for _, pr := range prs {
		pipelineBasic, err := c.ofPipelineBasic(ctx, pr)
		if err != nil {
			return nil, err
		}
		pipelineBasics = append(pipelineBasics, pipelineBasic)
	}
	return pipelineBasics, nil
}