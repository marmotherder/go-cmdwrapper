package cmdwrapper

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type logger interface {
	Infof(template string, args ...any)
	Error(args ...any)
	Errorf(template string, args ...any)
}

type CMDWrapper struct {
	Dir    string
	Logger logger
}

func (w CMDWrapper) RunCommand(name string, arg ...string) (*string, *int, error) {
	w.Logger.Infof("running command: %s %s in %s\n", name, arg, w.Dir)

	cmd := exec.Command(name, arg...)

	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	cmd.Dir = w.Dir
	if err := cmd.Start(); err != nil {
		w.Logger.Error("running command failed")
		w.Logger.Error(err.Error())
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr != nil {
				w.Logger.Error(string(exitErr.Stderr))
			}
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
				w.Logger.Errorf("exited with code %d\n", exitCode)
			}
		}
		return nil, &exitCode, err
	}

	scannerStdOut := bufio.NewScanner(stdOut)
	sbStdOut := strings.Builder{}
	go func() {
		for scannerStdOut.Scan() {
			sbStdOut.WriteString(fmt.Sprintf("%s\n", scannerStdOut.Text()))
		}
	}()

	scannerStdErr := bufio.NewScanner(stdErr)
	sbStdErr := strings.Builder{}
	go func() {
		for scannerStdErr.Scan() {
			sbStdErr.WriteString(fmt.Sprintf("%s\n", strings.TrimSpace(scannerStdErr.Text())))
		}
	}()

	cmd.Wait()

	stdOutString := strings.TrimSpace(sbStdOut.String())

	if sbStdErr.String() != "" {
		w.Logger.Error(sbStdErr.String())
	}

	w.Logger.Infof("exited with code %d\n", cmd.ProcessState.ExitCode())

	exitCode := cmd.ProcessState.ExitCode()
	return &stdOutString, &exitCode, nil
}

func (w CMDWrapper) RunCommandAsync(dir, name string, arg ...string) (chan string, chan string, func() error, *os.ProcessState, error) {
	w.Logger.Infof("running command: %s %s in %s\n", name, arg, dir)

	cmd := exec.Command(name, arg...)

	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		w.Logger.Error("running command failed")
		w.Logger.Error(err.Error())
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr != nil {
				w.Logger.Error(string(exitErr.Stderr))
			}
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
				w.Logger.Errorf("exited with code %d\n", exitCode)
			}
		}
		return nil, nil, nil, cmd.ProcessState, err
	}

	cso := make(chan string)
	scannerStdOut := bufio.NewScanner(stdOut)
	go func() {
		for scannerStdOut.Scan() {
			cso <- fmt.Sprintf("%s\n", scannerStdOut.Text())
		}
	}()

	cse := make(chan string)
	scannerStdErr := bufio.NewScanner(stdErr)
	go func() {
		for scannerStdErr.Scan() {
			cse <- fmt.Sprintf("%s\n", strings.TrimSpace(scannerStdErr.Text()))
		}
	}()

	cleandown := func() error {
		defer close(cso)
		defer close(cse)

		return cmd.Wait()
	}

	return cso, cse, cleandown, cmd.ProcessState, nil
}
