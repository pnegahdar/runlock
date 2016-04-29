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
var flagCommandOnAcquire = cli.StringFlag{Name: "on-acquire",
	Usage: "The command to run when lock is acquired."}
var flagCommandOnRelease = cli.StringFlag{Name: "on-release",
	Usage: "The command to run when lock is released."}
var flagValue = cli.StringFlag{Name: "value",
	Usage: "The identifier of this node for acquiring the lock"}

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

func runCommand(cmdToRun string) error {
	fmt.Println("Running command:", cmdToRun)
	command := safeShellSplit(cmdToRun)
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	return err
}

func runLoop(myID, etcdEndpoint string, lockKey string, ttl int, heartbeat int, acquireCommand, releaseCommand string) {
	hbDur := time.Second * time.Duration(heartbeat)
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
	acquireSignal := make(chan bool, 1)
	releaseSignal := make(chan bool, 1)


	// Lock loop
	go func() {
		haveLock := false
		loopedOnce := false
		for {
			setOptions := &client.SetOptions{PrevExist: client.PrevNoExist,
				TTL: time.Second * time.Duration(ttl)}
			kapi.Set(context.Background(), lockKey, myID, setOptions) // Error doesnt really matter here
			resp, err := kapi.Get(context.Background(), lockKey, nil)
			if err != nil {
				panic(err)
			}
			if resp.Node.Value == myID {
				// I have the lock
				now := time.Now().String()
				fmt.Println(now, "I have the lock. Key:", lockKey, "Value:", resp.Node.Value)
				if !haveLock {
					acquireSignal <- true
					haveLock = true
				}
				<-time.After(hbDur)
				setOptions := &client.SetOptions{PrevExist: client.PrevIgnore,
					PrevIndex: resp.Index,
					TTL: time.Second * time.Duration(ttl)}
				_, err = kapi.Set(context.Background(), lockKey, myID, setOptions)
				if err != nil {
					fmt.Println("Ran into error:", err.Error())
				}
			} else {
				now := time.Now().String()
				fmt.Println(now, "I dont have the lock. Key:", lockKey, "Value:", resp.Node.Value)
				if haveLock || ! loopedOnce {
					releaseSignal <- true
					haveLock = false
				}
				<-time.After(hbDur)
			}
			loopedOnce = true
		}
	}()
	runOrPanic := func(cmd string) {
		err := runCommand(cmd)
		if err != nil {
			panic(err)
		}
	}
	for {
		select {
		case <-acquireSignal:
			go runOrPanic(acquireCommand)
		case <-releaseSignal:
			go runOrPanic(releaseCommand)
		}
	}

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
			Flags:   []cli.Flag{flagEtcd, flagLockKey, flagTTL, flagHeartBeat, flagCommandOnAcquire, flagCommandOnRelease, flagValue},
			Action: func(c *cli.Context) {
				etcdHost := c.String(flagEtcd.Name)
				key := c.String(flagLockKey.Name)
				ttl := c.Int(flagTTL.Name)
				heartbeat := c.Int(flagHeartBeat.Name)
				commandOnAcquire := c.String(flagCommandOnAcquire.Name)
				commandOnRelease := c.String(flagCommandOnRelease.Name)
				lockValue := c.String(flagValue.Name)
				if lockValue == "" {
					lockValue = uuid.NewV4().String()
				}
				if heartbeat > ttl {
					fmt.Println("Heartbeat must be less than ttl.")
					os.Exit(1)
				}
				if commandOnAcquire == "" || key == "" || commandOnRelease == "" {
					fmt.Println("lock/release command  and key are required.")
					os.Exit(1)
				}
				runLoop(lockValue, etcdHost, key, ttl, heartbeat, commandOnAcquire, commandOnRelease)

			},
		},
	}
	app.Run(os.Args)
}

