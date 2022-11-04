package sandbox

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	dlc "github.com/gadget-inc/dateilager/pkg/client"
	"go.uber.org/zap"
)

const (
	NEXT_PROCESS_HEALTHY_INTERVAL = 500 * time.Millisecond
	OLD_PROCESS_GRACEFUL_INTERVAL = 2 * time.Second
	CHECK_LIVE_PORT_INTERVAL      = 100 * time.Millisecond

	MAX_PORT_OFFSET = 500
)

type Command struct {
	Exec    string
	Args    []string
	WorkDir string
}

func NewCommand(exec string, args []string, workDir string) Command {
	return Command{
		Exec:    exec,
		Args:    args,
		WorkDir: workDir,
	}
}

type Controller struct {
	Host string

	log        *zap.Logger
	command    Command
	portStart  int
	portOffset int
	project    int64
	dlClient   *dlc.Client
	cancelFunc context.CancelFunc

	procMutex sync.RWMutex
	counters  map[int]int
	current   *Process
	next      *Process
	gracefuls []*Process
}

func NewController(parentCtx context.Context, log *zap.Logger, host, dlServer string, project int64, command Command, portStart int) (*Controller, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	dlClient, err := dlc.NewClient(ctx, dlServer)
	if err != nil {
		cancel()
		return nil, err
	}

	controller := &Controller{
		Host:      host,
		log:       log,
		command:   command,
		portStart: portStart,
		project:   project,
		dlClient:  dlClient,

		cancelFunc: cancel,
		counters:   make(map[int]int),
	}

	go func() {
		client := &http.Client{}

		for {
			select {
			case <-ctx.Done():
				return
			default:
				nextPort := controller.getNextPort()
				if nextPort == -1 {
					time.Sleep(NEXT_PROCESS_HEALTHY_INTERVAL)
					continue
				}

				url := fmt.Sprintf("http://%s:%d/health", host, nextPort)
				resp, err := client.Get(url)
				if err != nil {
					log.Info("could not connect", zap.String("url", url))
					time.Sleep(NEXT_PROCESS_HEALTHY_INTERVAL)
					continue
				}

				if resp.StatusCode == http.StatusOK {
					log.Info("successful connection, upgrading to current", zap.String("url", url))
					controller.setCurrent(nextPort)
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				for _, oldProc := range controller.gracefuls {
					if controller.remainingRequests(oldProc.port) == 0 {
						oldProc.Kill()
						controller.removeGraceful(oldProc.port)
					}
				}
				time.Sleep(OLD_PROCESS_GRACEFUL_INTERVAL)
			}
		}
	}()

	return controller, nil
}

func (c *Controller) Close() {
	c.procMutex.Lock()
	defer c.procMutex.Unlock()

	if c.next != nil {
		c.next.Kill()
	}

	if c.current != nil {
		c.current.Kill()
	}

	for _, oldProc := range c.gracefuls {
		oldProc.Kill()
	}

	c.cancelFunc()
}

func (c *Controller) remainingRequests(port int) int {
	c.procMutex.RLock()
	defer c.procMutex.RUnlock()

	return c.counters[port]
}

func (c *Controller) removeGraceful(port int) {
	c.procMutex.Lock()
	defer c.procMutex.Unlock()

	for index, oldProc := range c.gracefuls {
		if oldProc.port == port {
			c.gracefuls = append(c.gracefuls[:index], c.gracefuls[index+1:]...)
			return
		}
	}
}

func (c *Controller) getNextPort() int {
	c.procMutex.RLock()
	defer c.procMutex.RUnlock()

	if c.next == nil {
		return -1
	}

	return c.next.port
}

func (c *Controller) killNextIfRunning() error {
	c.procMutex.Lock()
	defer c.procMutex.Unlock()

	if c.next != nil {
		err := c.next.Kill()
		if err != nil {
			return err
		}
		c.next = nil
	}

	return nil
}

func (c *Controller) setNext(proc *Process) {
	c.procMutex.Lock()
	defer c.procMutex.Unlock()

	c.next = proc
}

func (c *Controller) setCurrent(port int) {
	c.procMutex.Lock()
	defer c.procMutex.Unlock()

	// Test if we're attempting to set an older 'next' as 'current'.
	if c.next.port != port {
		return
	}

	if c.current != nil {
		c.gracefuls = append(c.gracefuls, c.current)
	}

	c.current = c.next
	c.next = nil
}

func (c *Controller) StartProcess(ctx context.Context, targetVersion *int64) (int64, error) {
	err := c.killNextIfRunning()
	if err != nil {
		return -1, fmt.Errorf("failed to kill concurrent next process: %w", err)
	}

	c.portOffset += 1
	if c.portOffset > MAX_PORT_OFFSET {
		c.portOffset = 0
	}
	port := c.portStart + c.portOffset

	version, _, err := c.dlClient.Rebuild(ctx, c.project, "", targetVersion, c.command.WorkDir, "/tmp")
	if err != nil {
		return -1, fmt.Errorf("failed to rebuild workdir to version %v: %w", version, err)
	}

	proc := NewProcess(c.log, c.command.Exec, c.command.Args[0], port, version)

	err = proc.Run(ctx)
	if err != nil {
		return -1, err
	}

	c.setNext(proc)
	return version, nil
}

func (c *Controller) IncrementRequestCounter(port int) {
	c.procMutex.Lock()
	defer c.procMutex.Unlock()

	c.counters[port] += 1
}

func (c *Controller) DecrementRequestCounter(port int) {
	c.procMutex.Lock()
	defer c.procMutex.Unlock()

	c.counters[port] -= 1

	if c.counters[port] == 0 {
		delete(c.counters, port)
	}
}

func (c *Controller) sendLivePort(portChan chan int) bool {
	c.procMutex.RLock()
	defer c.procMutex.RUnlock()

	if c.next == nil && c.current != nil {
		portChan <- c.current.port
		return true
	}
	return false
}

func (c *Controller) LivePortChannel(ctx context.Context) chan int {
	portChan := make(chan int, 1)
	foundPort := c.sendLivePort(portChan)

	if !foundPort {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if c.sendLivePort(portChan) {
						return
					} else {
						time.Sleep(CHECK_LIVE_PORT_INTERVAL)
					}
				}
			}
		}()
	}

	return portChan
}
