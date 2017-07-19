package ciel

import (
	"context"
	"errors"
	"io"
	"log"
	"os/exec"
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
	cmd := exec.Command("/usr/bin/systemd-nspawn", args...)
	c.Fs.lock.RUnlock()
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			c.lock.Lock()
			if c.booted {
				c.booted = false
				close(c.cancelBoot)
				c.cancelBoot = make(chan struct{})
			}
			c.lock.Unlock()
		}
	}()

	c.lock.Lock()
	defer c.lock.Unlock()

	for !c.isSystemRunning() {
		select {
		case <-c.cancelBoot:
			panic("container dead")
		default:
			time.Sleep(time.Millisecond * 100)
		}
	}
	c.booted = true
}

func (c *Container) isSystemRunning() bool {
	a, err := exec.Command("/usr/bin/systemctl", "is-system-running", "-M", c.Name).Output()
	if err != nil {
		switch string(a) {
		case "": // "Failed to connect to bus" => stderr, nothing in stdout.
			return false

		case "initializing", "starting", "offline":
			return false

		case "degraded":
			log.Printf("container: systemd is running in %s mode\n", string(a))
			return true

		case "maintenance", "unknown":
			close(c.cancelBoot)
			log.Printf("container: systemd is running in %s mode, stopping\n", string(a))
			return false

		case "stopping":
			close(c.cancelBoot)
			log.Println("container: systemd is stopping")
			return false
		}
	}
	return true
}

func (c *Container) isSystemShutdown() bool {
	return exec.Command("/usr/bin/machinectl", "status", c.Name).Run() != nil
}

func (c *Container) machinectlShutdown() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var cmd *exec.Cmd
	if c.booted {
		cmd = exec.Command("/usr/bin/machinectl", "poweroff", c.Name)
	} else if c.chrooted {
		cmd = exec.Command("/usr/bin/machinectl", "terminate", c.Name)
	} else {
		return nil
	}

	b, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(b))
	}

	for !c.isSystemShutdown() {
		time.Sleep(time.Millisecond * 100)
	}
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
		panic("container is down")
	}
	subArgs := append([]string{proc}, args...)
	subArgs = append([]string{
		"--quiet",
		"--wait",
		"--pty",
		"-M", c.Name,
	}, subArgs...)
	return cmd(ctx, "/usr/bin/systemd-run", stdin, stdout, stderr, subArgs...)
}

func (c *Container) systemdNspawnRun(ctx context.Context, proc string, stdin io.Reader, stdout, stderr io.Writer, args ...string) int {
	if c.IsActive() {
		panic("another chroot-mode instance is running")
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
	return cmd(ctx, "/usr/bin/systemd-nspawn", stdin, stdout, stderr, subArgs...)
}

func cmd(ctx context.Context, proc string, stdin io.Reader, stdout, stderr io.Writer, args ...string) int {
	cmd := exec.CommandContext(ctx, proc, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	err = cmd.Wait()
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		exitStatus := exitError.Sys().(syscall.WaitStatus)
		return exitStatus.ExitStatus()
	}
	panic(err)
}
