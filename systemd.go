package ciel

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func (c *Container) systemdNspawnBoot() {
	c.Fs.lock.RLock()
	args := []string{
		"--quiet",
		"--boot",
		"-M", c.Name,
		"-D", c.Fs.TargetDir(),
	}
	for _, p := range c.properties {
		args = append(args, "--property="+p)
	}
	dbglog.Println("systemdNspawnBoot: cmd <= /usr/bin/systemd-nspawn", args)
	cmd := exec.Command("/usr/bin/systemd-nspawn", args...)
	c.Fs.lock.RUnlock()
	infolog.Println("systemd-nspawn --boot")
	if err := cmd.Start(); err != nil {
		errlog.Panic(err)
	}
	go func() {
		dbglog.Println("systemdNspawnBoot: goroutine started: wait for process")
		if err := cmd.Wait(); err != nil {
			c.lock.Lock()
			if c.booted {
				c.booted = false
				close(c.cancelBoot)
				c.cancelBoot = make(chan struct{})
			}
			c.lock.Unlock()
			warnlog.Println("systemdNspawnBoot: cmd.Wait() => ", err)
		}
		dbglog.Println("systemdNspawnBoot: goroutine stopped")
	}()

	c.lock.Lock()
	defer c.lock.Unlock()

	infolog.Println("wait for booted...")
	for !c.isSystemRunning() {
		select {
		case <-c.cancelBoot:
			errlog.Panic("container dead")
		default:
			time.Sleep(time.Millisecond * 100)
		}
	}
	c.booted = true
	infolog.Println("wait for booted...OK")
}

func (c *Container) isSystemRunning() bool {
	a, err := exec.Command("/usr/bin/systemctl", "is-system-running", "-M", c.Name).Output()
	dbglog.Println("isSystemRunning:", err, strings.TrimSpace(string(a)))
	if err != nil {
		switch string(a) {
		case "": // "Failed to connect to bus" => stderr, nothing in stdout.
			return false

		case "initializing", "starting", "offline":
			return false

		case "degraded":
			warnlog.Printf("container: systemd is running in %s mode\n", strings.TrimSpace(string(a)))
			return true

		case "maintenance", "unknown":
			close(c.cancelBoot)
			errlog.Printf("container: systemd is running in %s mode, stopping\n", strings.TrimSpace(string(a)))
			return false

		case "stopping":
			close(c.cancelBoot)
			errlog.Println("container: systemd is stopping")
			return false
		}
	}
	return true
}

func (c *Container) isSystemShutdown() bool {
	err := exec.Command("/usr/bin/machinectl", "status", c.Name).Run()
	dbglog.Printf("isSystemShutdown: want err != nil, have err == %v\n", err)
	return err != nil
}

func (c *Container) machinectlShutdown() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var cmd *exec.Cmd
	if c.booted {
		dbglog.Println("machinectlShutdown: cmd <= /usr/bin/machinectl", "poweroff")
		cmd = exec.Command("/usr/bin/machinectl", "poweroff", c.Name)
	} else if c.chrooted {
		dbglog.Println("machinectlShutdown: cmd <= /usr/bin/machinectl", "terminate")
		cmd = exec.Command("/usr/bin/machinectl", "terminate", c.Name)
	} else {
		dbglog.Println("machinectlShutdown: no-op")
		return nil
	}

	a, err := cmd.CombinedOutput()
	if err != nil {
		dbglog.Println("machinectlShutdown: error", strings.TrimSpace(string(a)))
		return errors.New(string(a))
	}

	infolog.Println("wait for shutdown...")
	for !c.isSystemShutdown() {
		time.Sleep(time.Millisecond * 100)
	}
	infolog.Println("wait for shutdown...OK")
	c.booted = false
	close(c.cancelBoot)
	c.cancelBoot = make(chan struct{})
	return nil
}

func (c *Container) systemdRun(ctx context.Context, proc string, stdin io.Reader, stdout, stderr io.Writer, args ...string) int {
	c.lock.RLock()
	booted := c.booted
	c.lock.RUnlock()
	if !booted {
		errlog.Panic("container is down")
	}
	subArgs := append([]string{proc}, args...)
	subArgs = append([]string{
		"--quiet",
		"--wait",
		"--pty",
		"-M", c.Name,
	}, subArgs...)
	infolog.Println("systemd-run")
	return cmd(ctx, "/usr/bin/systemd-run", stdin, stdout, stderr, subArgs...)
}

func (c *Container) systemdNspawnRun(ctx context.Context, proc string, stdin io.Reader, stdout, stderr io.Writer, args ...string) int {
	if c.IsActive() {
		errlog.Panic("another chroot-mode instance is running")
	}

	subArgs := append([]string{proc}, args...)
	c.Fs.lock.RLock()
	subArgs = append([]string{
		"--quiet",
		"-M", c.Name,
		"-D", c.Fs.TargetDir(),
	}, subArgs...)
	c.Fs.lock.RUnlock()

	c.lock.Lock()
	c.chrooted = true
	c.lock.Unlock()
	defer func() {
		c.lock.Lock()
		c.chrooted = false
		c.lock.Unlock()
	}()
	infolog.Println("systemd-nspawn")
	return cmd(ctx, "/usr/bin/systemd-nspawn", stdin, stdout, stderr, subArgs...)
}

func cmd(ctx context.Context, proc string, stdin io.Reader, stdout, stderr io.Writer, args ...string) int {
	dbglog.Println("cmd:", proc, args)
	cmd := exec.CommandContext(ctx, proc, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Start()
	if err != nil {
		errlog.Panic(err)
	}
	err = cmd.Wait()
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		exitStatus := exitError.Sys().(syscall.WaitStatus)
		infolog.Println("exit status =", exitStatus.ExitStatus())
		return exitStatus.ExitStatus()
	}
	errlog.Panic(err)
	return 1
}
