package cli

import (
	"context"
	"os/exec"
)

// reduce boilerplate copy pasta with a factory method.
func baseCmd() *baseCommand {
	return &baseCommand{Ctx: context.Background()}
}

func PlanCmd() *PlanCommand {
	return &PlanCommand{baseCommand: baseCmd()}
}

func RunCmd() *RunCommand {
	return &RunCommand{baseCommand: baseCmd()}
}

func DestroyCmd() *DestroyCommand {
	return &DestroyCommand{&StopCommand{baseCommand: baseCmd()}}
}

func StatusCmd() *StatusCommand {
	return &StatusCommand{baseCommand: baseCmd()}
}

func StopCmd() *StopCommand {
	return &StopCommand{baseCommand: baseCmd()}
}

func NomadExec(args ...string) error {
	nomadPath, err := exec.LookPath("nomad")
	if err != nil {
		return err

	}

	nomadCmd := exec.Command(nomadPath, args...)
	err = nomadCmd.Run()
	return err
}
