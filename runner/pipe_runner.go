package runner

import (
	"context"
	"github.com/hanochg/piperika/runner/command"
	"time"
)

var (
	defaultBackoffConfig = backoffConfig{interval: time.Second, maxRetries: 30}

	cmds = []PipedCommand{
		NewRetryingPipedCommand("validate git state", command.New001ValidateGitState(), defaultBackoffConfig),
		NewRetryingPipedCommand("sync pipelines sources", command.New002PipelinesSourcesBranchSync(), defaultBackoffConfig),
		NewRetryingPipedCommand("find or trigger active run", command.New003PipelinesFindRun(), defaultBackoffConfig),
		NewRetryingPipedCommand("wait for run to finish", command.New004PipelinesWaitRun(), defaultBackoffConfig),
		NewRetryingPipedCommand("print run results", command.New005PipelinesPrintRun(), defaultBackoffConfig),
	}
)

func RunPipe(ctx context.Context) error {
	pipedState := &command.PipedCommandState{}
	for _, cmd := range cmds {
		err := cmd.Run(ctx, pipedState)
		if err != nil {
			return err
		}
	}
	return nil
}
