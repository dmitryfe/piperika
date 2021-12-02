package runs

import (
	"context"
	"fmt"
	"github.com/hanochg/piperika/http"
	"github.com/hanochg/piperika/http/models"
	"github.com/hanochg/piperika/http/requests"
	"github.com/hanochg/piperika/runner/datastruct"
	"github.com/hanochg/piperika/utils"
	"strconv"
	"strings"
	"time"
)

/*
	Run Description
	---------------
	- Get all the relevant runs
	- Check if there are relevant (on the same commit sha) active runs
	- Trigger a new run with "trigger_all"
*/

func (_ _03) Init(ctx context.Context, state *datastruct.PipedCommandState) (string, error) {
	state.RunId = -1
	state.RunNumber = -1
	return "", nil
}

func (_ _03) Tick(ctx context.Context, state *datastruct.PipedCommandState) (*datastruct.RunStatus, error) {
	httpClient := ctx.Value(utils.HttpClientCtxKey).(http.PipelineHttpClient)
	dirConfig := ctx.Value(utils.DirConfigCtxKey).(*utils.DirConfig)

	pipeResp, err := requests.GetPipelines(httpClient, models.GetPipelinesOptions{
		SortBy:     "latestRunId",
		FilterBy:   state.GitBranch,
		Light:      true,
		PipesNames: dirConfig.PipelineName,
	})
	if err != nil {
		return nil, err
	}
	if len(pipeResp.Pipelines) == 0 {
		return nil, fmt.Errorf("failed to get the pipeline")
	}
	state.PipelineId = pipeResp.Pipelines[0].PipelineId

	runResp, err := requests.GetRuns(httpClient, models.GetRunsOptions{
		PipelineIds: strconv.Itoa(state.PipelineId),
		Limit:       10,
		Light:       true,
		StatusCodes: strconv.Itoa(int(models.Processing)),
		SortBy:      "runNumber",
		SortOrder:   -1,
	})
	if err != nil {
		return nil, err
	}
	if len(runResp.Runs) == 0 {
		state.ShouldTriggerRun = true
		return &datastruct.RunStatus{
			Done: true,
		}, nil
	}

	runIds := make([]string, 0)
	for _, run := range runResp.Runs {
		runIds = append(runIds, strconv.Itoa(run.RunId))
	}
	runResourceResp, err := requests.GetRunResourceVersions(httpClient, models.GetRunResourcesOptions{
		PipelineSourceIds: strconv.Itoa(state.PipelinesSourceId),
		RunIds:            strings.Trim(strings.Join(runIds, ","), "[]"),
		SortBy:            "resourceTypeCode",
		SortOrder:         1,
	})
	if err != nil {
		return nil, err
	}

	if len(runResourceResp.Resources) == 0 {
		state.ShouldTriggerRun = true
		return &datastruct.RunStatus{
			Done: true,
		}, nil
	}

	activeRunIds := make([]int, 0)
	for _, runResource := range runResourceResp.Resources {
		if runResource.ResourceTypeCode != models.GitRepo {
			continue
		}
		if runResource.ResourceVersionContentPropertyBag.CommitSha == state.HeadCommitSha {
			activeRunIds = append(activeRunIds, runResource.RunId)
			break
		}
	}

	// Get the most recent run from the list
	for _, runIdStr := range runIds {
		runId, err := strconv.Atoi(runIdStr)
		if err != nil {
			return nil, err
		}
		if utils.Contains(activeRunIds, runId) {
			state.RunId = runId
			break
		}
	}

	if len(activeRunIds) != 0 && state.RunId != -1 {
		return &datastruct.RunStatus{
			Message: "Found an active run id",
			Done:    true,
		}, nil
	}

	state.ShouldTriggerRun = true
	return &datastruct.RunStatus{
		Message: "Triggering a new run",
		Done:    true,
	}, nil

}

func (_ _03) OnComplete(ctx context.Context, state *datastruct.PipedCommandState, status *datastruct.RunStatus) (string, error) {
	httpClient := ctx.Value(utils.HttpClientCtxKey).(http.PipelineHttpClient)

	if state.ShouldTriggerRun {
		pipeSteps, err := requests.GetPipelinesSteps(httpClient, models.GetPipelinesStepsOptions{
			PipelineIds:       strconv.Itoa(state.PipelineId),
			PipelineSourceIds: strconv.Itoa(state.PipelinesSourceId),
			Names:             utils.DefaultPipelinesStepNameToTrigger,
		})

		if err != nil {
			return "", err
		}
		if len(pipeSteps.Steps) == 0 {
			return "", fmt.Errorf("tried to trigger a run for step '%s' but coulnd't fetch its Id", utils.DefaultPipelinesStepNameToTrigger)
		}

		err = requests.TriggerPipelinesStep(httpClient, pipeSteps.Steps[0].Id)
		if err != nil {
			return "", err
		}

		// Giving Pipelines time to digest the request and create a new run
		time.Sleep(3 * time.Second)

		runResp, err := requests.GetRuns(httpClient, models.GetRunsOptions{
			PipelineIds: strconv.Itoa(state.PipelineId),
			Limit:       1,
			Light:       true,
			SortBy:      "createdAt",
			SortOrder:   -1,
		})
		if err != nil {
			return "", err
		}
		if len(runResp.Runs) == 0 {
			return "", fmt.Errorf("failed to get the triggered run")
		}
		state.RunId = runResp.Runs[0].RunId
		state.RunNumber = runResp.Runs[0].RunNumber
	}

	if state.RunNumber == -1 {
		runResp, err := requests.GetRuns(httpClient, models.GetRunsOptions{
			RunIds: strconv.Itoa(state.RunId),
		})
		if err != nil {
			return "", err
		}
		if len(runResp.Runs) == 0 {
			return "", fmt.Errorf("failed to get the triggered run")
		}
		state.RunNumber = runResp.Runs[0].RunNumber
	}

	return "", nil
}

type _03 struct {
}

func New003GetRun() datastruct.Runner {
	return _03{}
}
