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

func subprocess(startSignal, stopSignal chan bool, cmdToRun string) {
	command := []string{"/usr/bin/env", "sh", "-c", cmdToRun}
	var cmd *exec.Cmd
	for {
		select {
		case <-startSignal:
			fmt.Println("Starting the process.")
			cmd = exec.Command(command[0], command[1:]...)
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			go cmd.Run()
		case <-stopSignal:
			fmt.Println("Killing the process.")
			if cmd != nil {
				cmd.Process.Kill()
				cmd = nil
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
	startSignal := make(chan bool)
	stopSignal := make(chan bool)

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
	subprocess(startSignal, stopSignal, command)
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
