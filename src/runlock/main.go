package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/satori/go.uuid"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"os"
	"os/exec"
	"strings"
	"time"
	"os/signal"
	"syscall"
)

var flagEtcd = cli.StringFlag{Name: "etcd",
	Value: "http://localhost:2379",
	EnvVar: "RUNLOCK_ETCD_ENDPOINT",
	Usage: "The etcd enpoint to lock on"}
var flagLockKey = cli.StringFlag{Name: "key",
	EnvVar: "RUNLOCK_LOCK_KEY",
	Usage: "The key to use to lock the service."}
var flagTTL = cli.IntFlag{Name: "ttl",
	Value: 5, EnvVar: "RUNLOCK_TTL",
	Usage: "The TTL in seconds for the lock"}
var flagHeartBeat = cli.IntFlag{Name: "heartbeat",
	Value: 3, EnvVar: "RUNLOCK_HEARTBEAT",
	Usage: "How often to renew the lock"}
var flagCommand = cli.StringFlag{Name: "command",
	Usage: "The command to run, like 'echo hello'"}

const keyprefix = "/runlock/locks"

func addPrefix(key string) string {
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	return keyprefix + key
}

// https://gist.github.com/jmervine/d88c75329f98e09f5c87
func safeShellSplit(s string) []string {
	split := strings.Split(s, " ")

	var result []string
	var inquote string
	var block string
	for _, i := range split {
		if inquote == "" {
			if strings.HasPrefix(i, "'") || strings.HasPrefix(i, "\"") {
				inquote = string(i[0])
				block = strings.TrimPrefix(i, inquote) + " "
			} else {
				result = append(result, i)
			}
		} else {
			if !strings.HasSuffix(i, inquote) {
				block += i + " "
			} else {
				block += strings.TrimSuffix(i, inquote)
				inquote = ""
				result = append(result, block)
				block = ""
			}
		}
	}

	return result
}

func subprocess(startSignal, stopSignal chan bool, osSignal chan os.Signal, cmdToRun string) {
	command := safeShellSplit(cmdToRun)
	var cmd *exec.Cmd
	for {
		select {
		case <-startSignal:
			if cmd != nil {
				continue
			}
			fmt.Println("Starting the process with command:")
			fmt.Printf("%v", command)
			cmd = exec.Command(command[0], command[1:]...)
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			go func() {
				err := cmd.Run()
				if err != nil {
					panic(err)
				}
			}()
		case <-stopSignal:
			fmt.Println("Killing the process.")
			if cmd != nil {
				cmd.Process.Kill()
			}
		case sig := <-osSignal:
			fmt.Println("Sending singal to the process:", sig)
			if cmd != nil {
				err := cmd.Process.Signal(sig)
				if err != nil {
					panic(err)
				}
			}
			if sig == syscall.SIGTERM || sig == syscall.SIGINT {
				os.Exit(1)
			}
		}
	}
}

func runLoop(etcdEndpoint string, lockKey string, ttl int, heartbeat int, command string) {
	hbDur := time.Second * time.Duration(heartbeat)
	myUUID := uuid.NewV4().String()
	cfg := client.Config{
		Endpoints: []string{etcdEndpoint},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := client.New(cfg)
	if err != nil {
		panic(err)
	}
	lockKey = addPrefix(lockKey)
	kapi := client.NewKeysAPI(c)
	startSignal := make(chan bool, 1)
	stopSignal := make(chan bool, 1)


	// Lock loop
	go func() {
		haveLock := false
		for {
			setOptions := &client.SetOptions{PrevExist: client.PrevNoExist,
				TTL: time.Second * time.Duration(ttl)}
			kapi.Set(context.Background(), lockKey, myUUID, setOptions) // Error doesnt really matter here
			resp, err := kapi.Get(context.Background(), lockKey, nil)
			if err != nil {
				panic(err)
			}
			if resp.Node.Value == myUUID {
				// I have the lock
				fmt.Println("I have the lock. Key:", lockKey)
				if !haveLock {
					haveLock = true
					startSignal <- true
				}
				<-time.After(hbDur)
				setOptions := &client.SetOptions{PrevExist: client.PrevIgnore,
					PrevIndex: resp.Index,
					TTL: time.Second * time.Duration(ttl)}
				kapi.Set(context.Background(), lockKey, myUUID, setOptions)
			} else {
				fmt.Println("I dont have the lock. Key:", lockKey)
				if haveLock {
					haveLock = false
					stopSignal <- true
				}
				<-time.After(hbDur)
			}
		}
	}()
	// Forward signals
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	subprocess(startSignal, stopSignal, osSignal, command)
}

func main() {
	app := cli.NewApp()
	app.Name = "runlock"
	app.Usage = "Locks running things."
	app.Commands = []cli.Command{
		{
			Name:    "run",
			Aliases: []string{"r"},
			Usage:   "Run the locker.",
			Flags:   []cli.Flag{flagEtcd, flagLockKey, flagTTL, flagHeartBeat, flagCommand},
			Action: func(c *cli.Context) {
				etcdHost := c.String(flagEtcd.Name)
				key := c.String(flagLockKey.Name)
				ttl := c.Int(flagTTL.Name)
				heartbeat := c.Int(flagHeartBeat.Name)
				command := c.String(flagCommand.Name)
				if heartbeat > ttl {
					fmt.Println("Heartbeat must be less than ttl.")
					os.Exit(1)
				}
				if command == "" || key == "" {
					fmt.Println("Command  and key are required.")
					os.Exit(1)
				}
				runLoop(etcdHost, key, ttl, heartbeat, command)

			},
		},
	}
	app.Run(os.Args)
}
